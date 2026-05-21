// @MX:ANCHOR: 인증 헬퍼 — 쿠키 상수 + 세션 디코딩 진입점.
// @MX:REASON: 모든 BFF Route Handler 및 middleware가 본 상수/헬퍼를 사용 (fan_in >= 5).
// SPEC-AX-WEB-001 §7.2 HttpOnly 쿠키 BFF 방식 (OPEN #3 RESOLVED).

import { cookies } from "next/headers";
import { decodeJwt } from "jose";

import type { Role, UserSession } from "@/types/auth";

/**
 * 인증 쿠키 이름 — HttpOnly·Secure·SameSite=Lax 보장.
 * 클라이언트 JS는 본 쿠키에 접근 불가 (XSS 토큰 탈취 방어).
 */
export const COOKIE_ACCESS_TOKEN = "ax_access_token";
export const COOKIE_REFRESH_TOKEN = "ax_refresh_token";
export const COOKIE_CODE_VERIFIER = "ax_pkce_verifier";

/** PKCE state 쿠키 — CSRF 방어용 1회용 토큰 */
export const COOKIE_OIDC_STATE = "ax_oidc_state";

/**
 * Keycloak 환경 변수 묶음.
 * 누락 시 즉시 fail-fast — 환경 구성 실수 조기 발견.
 */
export function getKeycloakConfig(): {
  baseUrl: string;
  realm: string;
  clientId: string;
  appUrl: string;
} {
  const baseUrl = process.env["KEYCLOAK_BASE_URL"];
  const realm = process.env["KEYCLOAK_REALM"];
  const clientId = process.env["KEYCLOAK_CLIENT_ID"];
  const appUrl = process.env["NEXT_PUBLIC_APP_URL"];

  if (!baseUrl || !realm || !clientId || !appUrl) {
    throw new Error(
      "Keycloak 환경 변수 누락: KEYCLOAK_BASE_URL/REALM/CLIENT_ID, NEXT_PUBLIC_APP_URL 확인 필요",
    );
  }

  return { baseUrl, realm, clientId, appUrl };
}

/** Go control-plane base URL — BFF의 백엔드 호출 대상 */
export function getBackendBaseUrl(): string {
  return process.env["BACKEND_BASE_URL"] ?? "http://localhost:8080";
}

/**
 * access token에서 사용자 세션 메타데이터 추출.
 *
 * 본 함수는 JWT 서명을 검증하지 않는다 — 검증은 백엔드 RBAC 미들웨어에 위임.
 * UI 가시성 제어용 페이로드 디코드만 수행.
 *
 * @returns 디코드 성공 시 UserSession, 실패 시 null
 */
export function decodeAccessToken(token: string): UserSession | null {
  try {
    const claims = decodeJwt(token);

    const role = extractRole(claims["scope"]);
    if (!role) return null;

    const sub = typeof claims.sub === "string" ? claims.sub : null;
    const exp = typeof claims.exp === "number" ? claims.exp : null;
    if (!sub || exp === null) return null;

    const email =
      typeof claims["email"] === "string" ? claims["email"] : null;
    const name =
      typeof claims["preferred_username"] === "string"
        ? claims["preferred_username"]
        : typeof claims["name"] === "string"
          ? claims["name"]
          : null;

    return { sub, email, name, role, exp };
  } catch {
    return null;
  }
}

/**
 * Keycloak scope 클레임에서 iroum-ax 역할 1개 추출.
 * scope 형식: `"openid profile iroum-ax:analyst ..."`.
 * 다중 역할 부여 시 admin > analyst > viewer 우선순위.
 */
function extractRole(scope: unknown): Role | null {
  if (typeof scope !== "string") return null;

  const tokens = scope.split(/\s+/);
  if (tokens.includes("iroum-ax:admin")) return "admin";
  if (tokens.includes("iroum-ax:analyst")) return "analyst";
  if (tokens.includes("iroum-ax:viewer")) return "viewer";
  return null;
}

/**
 * 서버 컴포넌트/Route Handler에서 현재 세션 조회.
 * 쿠키가 없거나 만료된 경우 null.
 */
export async function getServerSession(): Promise<UserSession | null> {
  const cookieStore = await cookies();
  const accessToken = cookieStore.get(COOKIE_ACCESS_TOKEN)?.value;
  if (!accessToken) return null;

  const session = decodeAccessToken(accessToken);
  if (!session) return null;

  // 만료된 토큰은 세션 없음으로 간주 (refresh는 BFF refresh route에 위임)
  const nowSec = Math.floor(Date.now() / 1000);
  if (session.exp <= nowSec) return null;

  return session;
}

/**
 * 쿠키 설정 시 공통 옵션.
 * Phase A 기준 — 운영 배포 시 secure: true 강제 (NODE_ENV=production)
 */
export function getAuthCookieOptions(maxAgeSec: number): {
  httpOnly: true;
  secure: boolean;
  sameSite: "lax";
  path: string;
  maxAge: number;
} {
  return {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: maxAgeSec,
  };
}
