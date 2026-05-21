// @MX:NOTE: BFF 점수 상세/수정 프록시 — ScoreForm이 PUT 시 호출.
// SPEC-AX-WEB-001 §1.3 GET/PUT /api/v1/scores/{id} 정합.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * 인증 헤더 빌드 + ID 형식 검증 통합 가드.
 */
async function guard(
  id: string,
): Promise<
  | { ok: true; authorization: string }
  | { ok: false; response: Response }
> {
  if (!/^[A-Za-z0-9_-]{1,64}$/.test(id)) {
    return {
      ok: false,
      response: NextResponse.json(
        {
          error: {
            code: "INVALID_SCORE_ID",
            message: "잘못된 점수 ID 형식입니다.",
          },
        },
        { status: 400 },
      ),
    };
  }

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
 * GET /api/v1/scores/{id}
 */
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ id: string }> },
): Promise<Response> {
  const { id } = await context.params;
  const g = await guard(id);
  if (!g.ok) return g.response;

  const targetUrl = new URL(
    `/api/v1/scores/${encodeURIComponent(id)}`,
    getBackendBaseUrl(),
  );

  const upstream = await fetch(targetUrl.toString(), {
    method: "GET",
    headers: { Authorization: g.authorization },
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

/**
 * PUT /api/v1/scores/{id}
 *
 * 본문 형식: { value: number, comment?: string }.
 * eval_item_id는 immutable — 백엔드가 거부.
 */
export async function PUT(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
): Promise<Response> {
  const { id } = await context.params;
  const g = await guard(id);
  if (!g.ok) return g.response;

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

  const targetUrl = new URL(
    `/api/v1/scores/${encodeURIComponent(id)}`,
    getBackendBaseUrl(),
  );

  const upstream = await fetch(targetUrl.toString(), {
    method: "PUT",
    headers: {
      Authorization: g.authorization,
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
