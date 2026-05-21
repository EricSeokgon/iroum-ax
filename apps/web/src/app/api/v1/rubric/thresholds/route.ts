// @MX:NOTE: BFF 루브릭 임계값 list/create 프록시 — admin 전용 RubricThresholdTable/Form이 호출.
// SPEC-AX-WEB-001 Phase F REQ-WEB-008 + 백엔드 SPEC-AX-RUBRIC-001 정합.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * GET /api/v1/rubric/thresholds — 전체 임계값 목록 반환.
 */
export async function GET(_request: NextRequest): Promise<Response> {
  const cookieStore = await cookies();
  const accessToken = cookieStore.get(COOKIE_ACCESS_TOKEN)?.value;
  if (!accessToken) {
    return NextResponse.json(
      { error: { code: "UNAUTHENTICATED", message: "인증이 필요합니다." } },
      { status: 401 },
    );
  }

  const targetUrl = new URL("/api/v1/rubric/thresholds", getBackendBaseUrl());
  const upstream = await fetch(targetUrl.toString(), {
    method: "GET",
    headers: { Authorization: `Bearer ${accessToken}` },
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

/**
 * POST /api/v1/rubric/thresholds — 신규 임계값 생성.
 * 본문 검증은 클라이언트 1차 + 백엔드 최종 — 본 프록시는 통과만.
 */
export async function POST(request: NextRequest): Promise<Response> {
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
  const targetUrl = new URL("/api/v1/rubric/thresholds", getBackendBaseUrl());

  const upstream = await fetch(targetUrl.toString(), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
    },
    body: bodyText,
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

async function passthroughJson(upstream: Response): Promise<Response> {
  const status = upstream.status;
  if (status === 204) {
    return new NextResponse(null, { status: 204 });
  }

  const text = await upstream.text();
  if (!text) {
    return NextResponse.json({}, { status });
  }

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
