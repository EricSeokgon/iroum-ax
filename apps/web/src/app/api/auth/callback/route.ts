// @MX:ANCHOR: BFF callback route — Keycloak authorization code → 토큰 교환 + 쿠키 설정.
// @MX:REASON: 인증 흐름의 핵심 turning point. PKCE state 검증 + 백엔드 토큰 교환 단일 경로.
// SPEC-AX-WEB-001 REQ-WEB-001 + §7.2 단계 2~4.

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import type { NextRequest } from "next/server";

import {
  COOKIE_ACCESS_TOKEN,
  COOKIE_CODE_VERIFIER,
  COOKIE_OIDC_STATE,
  COOKIE_REFRESH_TOKEN,
  getAuthCookieOptions,
  getBackendBaseUrl,
  getKeycloakConfig,
} from "@/lib/auth";
import type { AuthTokenResponse } from "@/types/auth";

/**
 * GET /api/auth/callback?code=...&state=...
 *
 * 1. state 쿠키와 일치 검증 (CSRF 방어)
 * 2. code_verifier 쿠키 추출 (PKCE)
 * 3. Go control-plane POST /api/v1/auth/token 호출
 * 4. access/refresh 토큰을 HttpOnly 쿠키에 저장
 * 5. /dashboard로 redirect
 */
export async function GET(request: NextRequest): Promise<Response> {
  const code = request.nextUrl.searchParams.get("code");
  const state = request.nextUrl.searchParams.get("state");
  const error = request.nextUrl.searchParams.get("error");

  if (error) {
    // Keycloak이 명시적 오류 코드를 반환한 경우 로그인 화면으로 안전 복귀
    redirect(`/login?error=${encodeURIComponent(error)}`);
  }

  if (!code || !state) {
    redirect("/login?error=missing_code_or_state");
  }

  const cookieStore = await cookies();
  const expectedState = cookieStore.get(COOKIE_OIDC_STATE)?.value;
  const codeVerifier = cookieStore.get(COOKIE_CODE_VERIFIER)?.value;

  if (!expectedState || expectedState !== state) {
    redirect("/login?error=state_mismatch");
  }
  if (!codeVerifier) {
    redirect("/login?error=missing_verifier");
  }

  const config = getKeycloakConfig();

  // SPEC §1.3 카탈로그 정합 — Go control-plane이 Keycloak과 토큰 교환 수행
  const tokenResponse = await fetch(
    `${getBackendBaseUrl()}/api/v1/auth/token`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        code,
        code_verifier: codeVerifier,
        redirect_uri: `${config.appUrl}/api/auth/callback`,
        client_id: config.clientId,
      }),
      cache: "no-store",
    },
  );

  if (!tokenResponse.ok) {
    redirect("/login?error=token_exchange_failed");
  }

  const tokens = (await tokenResponse.json()) as AuthTokenResponse;

  // 1회용 PKCE 쿠키 제거 + 인증 쿠키 설정
  cookieStore.delete(COOKIE_OIDC_STATE);
  cookieStore.delete(COOKIE_CODE_VERIFIER);

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

  redirect("/dashboard");
}
