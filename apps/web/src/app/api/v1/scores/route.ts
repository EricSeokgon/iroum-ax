// @MX:ANCHOR: BFF 점수 list/create 프록시 — ScoreList/ScoreForm/scores page 3+ fan_in.
// @MX:REASON: ScoreList GET, ScoreForm POST, /dashboard/scores 페이지 GET (fan_in >= 3).
// SPEC-AX-WEB-001 §1.3 GET/POST /api/v1/scores 정합.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * 인증 헤더 빌드 헬퍼 — access token이 없으면 401 응답을 즉시 반환할 수 있도록 가공.
 */
async function authorizationOrUnauthorized(): Promise<
  | { ok: true; authorization: string }
  | { ok: false; response: Response }
> {
  const cookieStore = await cookies();
  const accessToken = cookieStore.get(COOKIE_ACCESS_TOKEN)?.value;

  if (!accessToken) {
    return {
      ok: false,
      response: NextResponse.json(
        { error: { code: "UNAUTHENTICATED", message: "인증이 필요합니다." } },
        { status: 401 },
      ),
    };
  }

  return { ok: true, authorization: `Bearer ${accessToken}` };
}

/**
 * GET /api/v1/scores?eval_item_id=&limit=&offset=
 *
 * eval_item_id로 특정 평가항목의 점수만 필터링 가능 (생략 시 전체).
 */
export async function GET(request: NextRequest): Promise<Response> {
  const auth = await authorizationOrUnauthorized();
  if (!auth.ok) return auth.response;

  const url = new URL(request.url);
  const targetUrl = new URL("/api/v1/scores", getBackendBaseUrl());

  // 화이트리스트 파라미터만 통과 — 백엔드 표면 최소 노출
  const evalItemId = url.searchParams.get("eval_item_id");
  const limit = url.searchParams.get("limit");
  const offset = url.searchParams.get("offset");

  if (evalItemId !== null) {
    // path traversal/injection 방어 — ID 형식 가드
    if (!/^[A-Za-z0-9_-]{1,64}$/.test(evalItemId)) {
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
    targetUrl.searchParams.set("eval_item_id", evalItemId);
  }
  if (limit !== null) targetUrl.searchParams.set("limit", limit);
  if (offset !== null) targetUrl.searchParams.set("offset", offset);

  const upstream = await fetch(targetUrl.toString(), {
    method: "GET",
    headers: { Authorization: auth.authorization },
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

/**
 * POST /api/v1/scores (application/json)
 *
 * 본문 형식: { eval_item_id: string, value: number, comment?: string }.
 * 본문은 텍스트로 받아 백엔드에 그대로 전달 — 형식 검증은 백엔드에 위임한다.
 */
export async function POST(request: NextRequest): Promise<Response> {
  const auth = await authorizationOrUnauthorized();
  if (!auth.ok) return auth.response;

  const contentType = request.headers.get("content-type") ?? "";
  if (!contentType.includes("application/json")) {
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

  const body = await request.text();
  if (!body) {
    return NextResponse.json(
      {
        error: {
          code: "EMPTY_BODY",
          message: "요청 본문이 비어 있습니다.",
        },
      },
      { status: 400 },
    );
  }

  const targetUrl = new URL("/api/v1/scores", getBackendBaseUrl());

  const upstream = await fetch(targetUrl.toString(), {
    method: "POST",
    headers: {
      Authorization: auth.authorization,
      "Content-Type": "application/json",
    },
    body,
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

/**
 * 백엔드 응답을 그대로 클라이언트에 전달하면서 JSON 본문은 보존.
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
