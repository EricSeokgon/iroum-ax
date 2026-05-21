// @MX:ANCHOR: 인증 도메인 타입 — UI/BFF/middleware 전반에서 공유.
// @MX:REASON: UserSession/Role은 RoleGate/middleware/API 클라이언트 등 fan_in >= 3 경로.
// SPEC-AX-WEB-001 §1.4 RBAC 3-역할 + §7.2 인증 흐름 정합.

/**
 * SPEC-AX-AUTH-001 scope 클레임 형식: `iroum-ax:{admin|analyst|viewer}`.
 * 백엔드 RBAC가 신뢰의 원천이며, UI는 가시성 제어 용도로만 사용한다.
 */
export type Role = "admin" | "analyst" | "viewer";

/**
 * 인증된 사용자 세션 메타데이터.
 * access token 디코드 결과를 안전하게 노출하기 위한 축약형.
 */
export interface UserSession {
  /** Keycloak `sub` 클레임 — 사용자 고유 ID */
  sub: string;
  /** 이메일 (가용 시) */
  email: string | null;
  /** Keycloak preferred_username 또는 name */
  name: string | null;
  /** SPEC-AX-AUTH-002 RBAC 역할 (3-role) */
  role: Role;
  /** access token 만료 epoch (초). 토큰 갱신 스케줄링용 */
  exp: number;
}

/**
 * 백엔드 `/api/v1/auth/token` 응답 형식 (SPEC-AX-AUTH-001 정합).
 */
export interface AuthTokenResponse {
  access_token: string;
  refresh_token: string;
  token_type: "Bearer";
  expires_in: number;
  refresh_expires_in?: number;
}
