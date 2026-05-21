// @MX:ANCHOR: BFF login route — Keycloak OIDC authorization endpoint redirect.
// @MX:REASON: 로그인 화면이 단일 진입점으로 호출 (fan_in == 1이지만 보안 경계의 lone entry).
// SPEC-AX-WEB-001 REQ-WEB-001 + §7.2 단계 1.

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import type { NextRequest } from "next/server";

import {
  COOKIE_CODE_VERIFIER,
  COOKIE_OIDC_STATE,
  getAuthCookieOptions,
  getKeycloakConfig,
} from "@/lib/auth";
import {
  generateCodeChallenge,
  generateCodeVerifier,
  generateOidcState,
} from "@/lib/pkce";

/**
 * GET /api/auth/login — 사용자를 Keycloak Authorization Endpoint로 redirect.
 *
 * 1. PKCE code_verifier + state 생성 → HttpOnly 쿠키 보관 (10분 유효)
 * 2. code_challenge(S256) 계산
 * 3. Keycloak authorize URL 빌드 후 302 redirect
 */
export async function GET(_request: NextRequest): Promise<Response> {
  const config = getKeycloakConfig();

  const codeVerifier = generateCodeVerifier();
  const codeChallenge = await generateCodeChallenge(codeVerifier);
  const state = generateOidcState();

  const cookieStore = await cookies();
  // 10분 — Keycloak 기본 authorization code 유효시간과 정합
  const cookieOptions = getAuthCookieOptions(60 * 10);
  cookieStore.set(COOKIE_CODE_VERIFIER, codeVerifier, cookieOptions);
  cookieStore.set(COOKIE_OIDC_STATE, state, cookieOptions);

  const authUrl = new URL(
    `/realms/${config.realm}/protocol/openid-connect/auth`,
    config.baseUrl,
  );
  authUrl.searchParams.set("client_id", config.clientId);
  authUrl.searchParams.set("response_type", "code");
  authUrl.searchParams.set(
    "redirect_uri",
    `${config.appUrl}/api/auth/callback`,
  );
  authUrl.searchParams.set(
    "scope",
    "openid profile email iroum-ax:admin iroum-ax:analyst iroum-ax:viewer",
  );
  authUrl.searchParams.set("state", state);
  authUrl.searchParams.set("code_challenge", codeChallenge);
  authUrl.searchParams.set("code_challenge_method", "S256");

  redirect(authUrl.toString());
}
