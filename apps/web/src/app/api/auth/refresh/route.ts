// @MX:NOTE: BFF refresh route — 만료 임박 access token 자동 갱신.
// SPEC-AX-WEB-001 REQ-WEB-008 + §7.2 단계 7.

import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import {
  COOKIE_ACCESS_TOKEN,
  COOKIE_REFRESH_TOKEN,
  getAuthCookieOptions,
  getBackendBaseUrl,
} from "@/lib/auth";
import type { AuthTokenResponse } from "@/types/auth";

/**
 * POST /api/auth/refresh
 *
 * 클라이언트는 본 endpoint를 호출하기만 하면 됨 — 쿠키 자동 첨부.
 * 갱신 실패 시 401 반환 → 클라이언트는 /login redirect 트리거.
 */
export async function POST(): Promise<Response> {
  const cookieStore = await cookies();
  const refreshToken = cookieStore.get(COOKIE_REFRESH_TOKEN)?.value;

  if (!refreshToken) {
    return NextResponse.json(
      { error: { code: "MISSING_REFRESH_TOKEN", message: "갱신 토큰이 없습니다." } },
      { status: 401 },
    );
  }

  const response = await fetch(
    `${getBackendBaseUrl()}/api/v1/auth/refresh`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
      cache: "no-store",
    },
  );

  if (!response.ok) {
    // refresh token도 만료 → 인증 쿠키 모두 삭제
    cookieStore.delete(COOKIE_ACCESS_TOKEN);
    cookieStore.delete(COOKIE_REFRESH_TOKEN);
    return NextResponse.json(
      { error: { code: "REFRESH_FAILED", message: "세션이 만료되었습니다." } },
      { status: 401 },
    );
  }

  const tokens = (await response.json()) as AuthTokenResponse;

  cookieStore.set(
    COOKIE_ACCESS_TOKEN,
    tokens.access_token,
    getAuthCookieOptions(tokens.expires_in),
  );
  cookieStore.set(
    COOKIE_REFRESH_TOKEN,
    tokens.refresh_token,
    getAuthCookieOptions(tokens.refresh_expires_in ?? 60 * 60 * 8),
  );

  return NextResponse.json({ ok: true });
}
