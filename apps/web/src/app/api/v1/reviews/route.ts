// @MX:ANCHOR: BFF 리뷰 list/create 프록시 — KanbanBoard 새로고침과 SubmitReviewForm 제출 경유.
// @MX:REASON: KanbanBoard(client) refresh + SubmitReviewForm(client) submit + RSC page initial load (fan_in == 3).
// SPEC-AX-WEB-001 Phase E REQ-WEB-006 §7.2 BFF 패턴.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * Authorization 헤더 빌드 helper — 미인증 시 401 응답을 즉시 반환할 수 있도록 가공.
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
 * GET /api/v1/reviews
 *
 * 리뷰 전체 목록 — 클라이언트(KanbanBoard)가 4-칼럼 분류용으로 호출.
 * 페이지네이션은 PoC 범위에서 미지원(백엔드도 단일 페이지 반환 가정).
 */
export async function GET(_request: NextRequest): Promise<Response> {
  const auth = await authorizationOrUnauthorized();
  if (!auth.ok) return auth.response;

  const targetUrl = new URL("/api/v1/reviews", getBackendBaseUrl());

  const upstream = await fetch(targetUrl.toString(), {
    method: "GET",
    headers: { Authorization: auth.authorization },
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

/**
 * POST /api/v1/reviews
 *
 * 리뷰 제출 — analyst/admin만 호출 가능(백엔드 RBAC가 신뢰의 원천).
 * 본 핸들러는 역할 검증을 수행하지 않고 백엔드의 403을 그대로 전달한다.
 * 본문 형식: `{ title?: string, comment?: string }`.
 */
export async function POST(request: NextRequest): Promise<Response> {
  const auth = await authorizationOrUnauthorized();
  if (!auth.ok) return auth.response;

  const body = await safeReadJsonBody(request);
  if (body.kind === "error") {
    return NextResponse.json(
      { error: { code: "INVALID_BODY", message: body.message } },
      { status: 400 },
    );
  }

  const targetUrl = new URL("/api/v1/reviews", getBackendBaseUrl());

  const upstream = await fetch(targetUrl.toString(), {
    method: "POST",
    headers: {
      Authorization: auth.authorization,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body.value),
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

/**
 * JSON 본문을 안전하게 읽어 화이트리스트된 필드만 통과시킨다.
 * 백엔드 표면 노출 최소화 — 미지의 필드는 무시.
 */
async function safeReadJsonBody(
  request: NextRequest,
): Promise<
  | { kind: "ok"; value: { title?: string; comment?: string } }
  | { kind: "error"; message: string }
> {
  let raw: unknown;
  try {
    raw = await request.json();
  } catch {
    return {
      kind: "error",
      message: "JSON 본문을 해석할 수 없습니다.",
    };
  }
  if (raw === null || typeof raw !== "object") {
    return {
      kind: "error",
      message: "본문은 JSON 객체여야 합니다.",
    };
  }
  const record = raw as Record<string, unknown>;
  const value: { title?: string; comment?: string } = {};
  if (typeof record["title"] === "string") {
    value.title = record["title"];
  }
  if (typeof record["comment"] === "string") {
    value.comment = record["comment"];
  }
  return { kind: "ok", value };
}

/**
 * 백엔드 응답을 그대로 클라이언트에 전달하면서 JSON 본문은 보존.
 * 4xx/5xx도 그대로 통과 — 클라이언트가 error.code로 분기 가능.
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
