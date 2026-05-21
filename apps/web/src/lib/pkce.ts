// @MX:NOTE: OIDC PKCE 헬퍼 — Web Crypto 기반 code_verifier/code_challenge 생성.
// SPEC-AX-WEB-001 §7.2 + §7.8: Public client + PKCE S256, client_secret 미사용.

/**
 * RFC 7636 PKCE code_verifier 생성 — 43~128자 base64url-safe.
 * 본 구현은 32바이트 무작위 → base64url 변환 (43자).
 */
export function generateCodeVerifier(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return base64UrlEncode(bytes);
}

/**
 * PKCE S256 code_challenge 계산 — SHA-256(verifier) → base64url.
 */
export async function generateCodeChallenge(verifier: string): Promise<string> {
  const data = new TextEncoder().encode(verifier);
  const digest = await crypto.subtle.digest("SHA-256", data);
  return base64UrlEncode(new Uint8Array(digest));
}

/** CSRF 방어용 state 토큰 — 16바이트 무작위 → base64url */
export function generateOidcState(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return base64UrlEncode(bytes);
}

function base64UrlEncode(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}
