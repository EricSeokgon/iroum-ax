// @MX:NOTE: BFF me route — 클라이언트가 현재 사용자 정보를 안전하게 조회.
// SPEC-AX-WEB-001 §7.2 단계 5 — 클라이언트는 토큰 직접 접근 불가, role/name만 받음.

import { NextResponse } from "next/server";

import { getServerSession } from "@/lib/auth";

/**
 * GET /api/auth/me
 *
 * 인증된 사용자: 200 + UserSession (토큰 미포함)
 * 미인증: 401
 */
export async function GET(): Promise<Response> {
  const session = await getServerSession();

  if (!session) {
    return NextResponse.json(
      { error: { code: "UNAUTHENTICATED", message: "인증이 필요합니다." } },
      { status: 401 },
    );
  }

  return NextResponse.json(session);
}
