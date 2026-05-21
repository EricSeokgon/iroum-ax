// @MX:NOTE: BFF 감사 로그 list 프록시 — admin 전용 AuditLogTable이 호출.
// SPEC-AX-WEB-001 Phase F REQ-WEB-007 + 백엔드 SPEC-AX-AUDIT-QUERY-001 정합.
// 인가 자체는 백엔드 RBAC가 최종 결정 — 본 핸들러는 토큰만 위임.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * GET /api/v1/audit-logs
 *
 * Query params (whitelist): action, user_id, resource_type, start_time, end_time, limit, offset
 * — 미지 파라미터는 차단해 백엔드 표면 노출 최소화.
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
  const targetUrl = new URL("/api/v1/audit-logs", getBackendBaseUrl());

  // SPEC §1.3 catalog 정합 — 허용 query param 6종만 통과.
  const ALLOWED_PARAMS = [
    "action",
    "user_id",
    "resource_type",
    "start_time",
    "end_time",
    "limit",
    "offset",
  ] as const;
  for (const key of ALLOWED_PARAMS) {
    const value = url.searchParams.get(key);
    if (value !== null && value !== "") {
      targetUrl.searchParams.set(key, value);
    }
  }

  const upstream = await fetch(targetUrl.toString(), {
    method: "GET",
    headers: { Authorization: `Bearer ${accessToken}` },
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
