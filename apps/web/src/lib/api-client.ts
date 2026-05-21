// @MX:ANCHOR: API fetch wrapper — BFF/서버 컴포넌트가 사용하는 단일 진입점.
// @MX:REASON: 모든 백엔드 호출이 본 함수를 거쳐 인증 헤더·에러 정규화를 일관 적용 (fan_in >= 5).
// SPEC-AX-WEB-001 §1.3 카탈로그 31 endpoints의 통합 진입점.

import { cookies } from "next/headers";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * 백엔드 표준 에러 응답 형식 (SPEC-AX-AUTH-001 §6.8 정합).
 */
export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
    field?: string;
  };
}

/**
 * 사용자에게 표시 가능한 정규화된 API 에러.
 */
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    public override message: string,
    public field?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * 서버측 인증 fetch — 쿠키의 access token을 Authorization 헤더로 변환해 백엔드 호출.
 *
 * 본 wrapper는 RSC/Route Handler 등 서버 환경에서만 사용한다.
 * 클라이언트 컴포넌트는 BFF Route Handler를 경유해야 한다 (토큰 직접 접근 차단).
 *
 * @param path - `/api/v1/*` 형식 (선행 `/` 필수)
 * @returns 파싱된 JSON 응답
 * @throws ApiError - 4xx/5xx 응답 시
 */
export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  if (!path.startsWith("/api/v1/")) {
    throw new Error(
      `apiFetch path는 '/api/v1/'로 시작해야 합니다 — 실제: ${path}`,
    );
  }

  const cookieStore = await cookies();
  const accessToken = cookieStore.get(COOKIE_ACCESS_TOKEN)?.value;

  const headers = new Headers(init?.headers);
  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }
  if (!headers.has("Content-Type") && init?.body) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${getBackendBaseUrl()}${path}`, {
    ...init,
    headers,
    // 인증 쿠키 누설 방지 — 쿠키는 Authorization 헤더로만 전달
    credentials: "omit",
    cache: "no-store",
  });

  if (!response.ok) {
    let body: ApiErrorBody | undefined;
    try {
      body = (await response.json()) as ApiErrorBody;
    } catch {
      // JSON 파싱 실패 시 status 기반 fallback 메시지
    }

    throw new ApiError(
      response.status,
      body?.error.code ?? `HTTP_${response.status}`,
      body?.error.message ?? "백엔드 요청 처리 중 오류가 발생했습니다.",
      body?.error.field,
    );
  }

  // 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}
