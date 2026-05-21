// @MX:ANCHOR: BFF 증빙 list/upload 프록시 — 클라이언트가 Authorization 토큰을 직접 보지 않도록 경유.
// @MX:REASON: 업로드/목록 두 클라이언트 컴포넌트(EvidenceUpload, EvidenceList) + RSC page가 호출 (fan_in == 3).
// SPEC-AX-WEB-001 §7.2 BFF 패턴 + §1.3 evidences endpoints 정합.

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
 * GET /api/v1/evidences?limit=&offset=
 *
 * 클라이언트가 호출하면 본 핸들러가 쿠키에서 access token을 꺼내
 * Go control-plane으로 Bearer 토큰을 동봉해 프록시한다.
 */
export async function GET(request: NextRequest): Promise<Response> {
  const auth = await authorizationOrUnauthorized();
  if (!auth.ok) return auth.response;

  const url = new URL(request.url);
  const targetUrl = new URL("/api/v1/evidences", getBackendBaseUrl());
  // limit/offset만 통과 — 미지 파라미터는 차단해 백엔드 표면 노출 최소화
  const limit = url.searchParams.get("limit");
  const offset = url.searchParams.get("offset");
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
 * POST /api/v1/evidences (multipart/form-data)
 *
 * 업로드 본문은 Body 스트림으로 그대로 백엔드에 전달한다.
 * Content-Type(boundary 포함)을 변형하지 않도록 클라이언트의 헤더를 보존.
 *
 * @MX:NOTE: 100MB 클라이언트 사전 검증이 1차 보호선, 백엔드 multipart 한도가 최종 방어선.
 */
export async function POST(request: NextRequest): Promise<Response> {
  const auth = await authorizationOrUnauthorized();
  if (!auth.ok) return auth.response;

  const contentType = request.headers.get("content-type");
  if (!contentType || !contentType.startsWith("multipart/form-data")) {
    return NextResponse.json(
      {
        error: {
          code: "INVALID_CONTENT_TYPE",
          message: "multipart/form-data 형식의 요청만 허용됩니다.",
        },
      },
      { status: 415 },
    );
  }

  if (!request.body) {
    return NextResponse.json(
      {
        error: {
          code: "EMPTY_BODY",
          message: "업로드할 본문이 비어 있습니다.",
        },
      },
      { status: 400 },
    );
  }

  const targetUrl = new URL("/api/v1/evidences", getBackendBaseUrl());

  // request.body는 ReadableStream — undici/Node fetch가 duplex half를 요구하나
  // RequestInit typings(lib.dom.d.ts)에 duplex가 아직 없어 init 객체로 우회한다.
  const init: RequestInit = {
    method: "POST",
    headers: {
      Authorization: auth.authorization,
      "Content-Type": contentType,
    },
    body: request.body,
    cache: "no-store",
  };
  // @MX:NOTE: duplex 옵션은 Node.js streaming body 전송 필수 — typings 미반영.
  (init as RequestInit & { duplex?: "half" }).duplex = "half";

  const upstream = await fetch(targetUrl.toString(), init);

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
  // 본문이 비어있는 비-204 응답은 보수적으로 빈 객체로 처리
  if (!text) {
    return NextResponse.json({}, { status });
  }

  try {
    const json: unknown = JSON.parse(text);
    return NextResponse.json(json, { status });
  } catch {
    // JSON 파싱 실패 시 표준 에러 envelope으로 변환
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
