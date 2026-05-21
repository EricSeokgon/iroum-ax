// @MX:ANCHOR: RoleGate — 자식 요소의 조건부 렌더링 (RBAC UI 가시성).
// @MX:REASON: 모든 권한 분기 화면에서 사용 예정 (fan_in 향후 증가, Phase B 이후).
// SPEC-AX-WEB-001 §7.4 RBAC UI 가시성 패턴.

import type { Role } from "@/types/auth";

interface RoleGateProps {
  /** 현재 사용자 역할 */
  currentRole: Role;
  /** 허용 역할 목록 (allow-list) */
  allow: ReadonlyArray<Role>;
  /** 차단 시 표시할 fallback (기본: null) */
  fallback?: React.ReactNode;
  children: React.ReactNode;
}

/**
 * 보안 경계가 아닌 UX 보조 컴포넌트.
 * 백엔드 RBAC가 최종 인가의 신뢰의 원천이다 (SPEC §1.4).
 */
export function RoleGate({
  currentRole,
  allow,
  fallback = null,
  children,
}: RoleGateProps): React.ReactNode {
  if (allow.includes(currentRole)) {
    return children;
  }
  return fallback;
}
