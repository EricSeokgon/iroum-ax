// @MX:ANCHOR: BFF 평가항목 목록 프록시 — RSC + ItemTree 2개 이상 fan_in.
// @MX:REASON: page RSC가 초기 fetch, ItemTree가 클라이언트 측 새로고침 호출 (fan_in >= 3 예상).
// SPEC-AX-WEB-001 §1.3 GET /api/v1/evaluation-items 정합.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * GET /api/v1/evaluation-items
 *
 * 평탄화된 트리 노드 리스트를 반환한다 (parent_id로 계층 표현).
 * 클라이언트는 본 응답으로 트리를 재구성한다.
 *
 * 쿼리 파라미터는 백엔드 표면 노출을 최소화하기 위해 limit/offset만 통과시킨다.
 */
export async function GET(request: NextRequest): Promise<Response> {
  const cookieStore = await cookies();
  const accessToken = cookieStore.get(COOKIE_ACCESS_TOKEN)?.value;
  if (!accessToken) {
    return NextResponse.json(
      { error: { code: "UNAUTHENTICATED", message: "인증이 필요합니다." } },
      { status: 401 },
    );
  }

  const url = new URL(request.url);
  const targetUrl = new URL("/api/v1/evaluation-items", getBackendBaseUrl());
  const limit = url.searchParams.get("limit");
  const offset = url.searchParams.get("offset");
  if (limit !== null) targetUrl.searchParams.set("limit", limit);
  if (offset !== null) targetUrl.searchParams.set("offset", offset);

  const upstream = await fetch(targetUrl.toString(), {
    method: "GET",
    headers: { Authorization: `Bearer ${accessToken}` },
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

/**
 * 백엔드 응답을 그대로 클라이언트에 전달하면서 JSON 본문은 보존.
 * 4xx/5xx도 그대로 통과시켜 클라이언트가 error.code로 분기할 수 있게 한다.
 */
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
