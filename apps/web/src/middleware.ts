// @MX:ANCHOR: Next.js middleware — /dashboard/* 경로 보호 가드 (1차 방어선).
// @MX:REASON: 모든 보호 페이지가 본 가드를 거침 (fan_in == 전체 dashboard 경로).
// SPEC-AX-WEB-001 REQ-WEB-001b: 미인증 사용자의 보호 경로 접근 차단.

import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN } from "@/lib/auth";

/**
 * 인증 쿠키 존재 여부만 검사 — JWT 디코딩/검증은 페이지/Route Handler에 위임.
 *
 * Edge runtime 호환을 위해 jose 디코드는 본 미들웨어에서 수행하지 않는다.
 * 만료된 토큰도 통과시키되, RSC가 getServerSession()에서 null을 받아
 * redirect('/login')으로 추가 안전 복귀시킨다 (이중 방어).
 */
export function middleware(request: NextRequest): NextResponse {
  const accessToken = request.cookies.get(COOKIE_ACCESS_TOKEN)?.value;

  if (!accessToken) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("from", request.nextUrl.pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  // /dashboard/* 만 보호 — /login·/api/auth/* 는 공개
  matcher: ["/dashboard/:path*"],
};
