// @MX:NOTE: BFF logout route — 백엔드 토큰 무효화 + 모든 인증 쿠키 삭제.
// SPEC-AX-WEB-001 REQ-WEB-008a.

import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import {
  COOKIE_ACCESS_TOKEN,
  COOKIE_CODE_VERIFIER,
  COOKIE_OIDC_STATE,
  COOKIE_REFRESH_TOKEN,
  getBackendBaseUrl,
} from "@/lib/auth";

/**
 * POST /api/auth/logout
 *
 * 1. 백엔드 POST /api/v1/auth/logout 호출 (refresh token 무효화)
 * 2. 모든 인증·OIDC 쿠키 삭제
 * 3. 200 OK 반환 — 클라이언트는 /login으로 redirect
 *
 * 백엔드 호출 실패해도 클라이언트 쿠키는 반드시 삭제 (best-effort 로그아웃).
 */
export async function POST(): Promise<Response> {
  const cookieStore = await cookies();
  const refreshToken = cookieStore.get(COOKIE_REFRESH_TOKEN)?.value;
  const accessToken = cookieStore.get(COOKIE_ACCESS_TOKEN)?.value;

  if (refreshToken && accessToken) {
    try {
      await fetch(`${getBackendBaseUrl()}/api/v1/auth/logout`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${accessToken}`,
        },
        body: JSON.stringify({ refresh_token: refreshToken }),
        cache: "no-store",
      });
    } catch {
      // 네트워크 오류는 무시 — 클라이언트 쿠키 삭제가 우선
    }
  }

  cookieStore.delete(COOKIE_ACCESS_TOKEN);
  cookieStore.delete(COOKIE_REFRESH_TOKEN);
  cookieStore.delete(COOKIE_OIDC_STATE);
  cookieStore.delete(COOKIE_CODE_VERIFIER);

  return NextResponse.json({ ok: true });
}
