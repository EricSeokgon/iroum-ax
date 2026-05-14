// Package auth — SSO/JWT 인증 + 3-role RBAC 패키지
//
// SPEC-AX-AUTH-001 Sprint 0 Foundation stub.
// Sprint 1+ 에서 비즈니스 로직을 구현한다.
//
// 핵심 진입점:
//
//   - [TokenValidator.Verify] — 모든 인증 경로의 단일 진입점 (fan_in >= 5)
//   - [Authorize] — RBAC 결정 단일 진입점 (fan_in >= 4)
//
// @MX:ANCHOR: [AUTO] TokenValidator — 모든 인증 경로 단일 진입점
// @MX:REASON: gRPC interceptor / REST middleware / logout / refresh / test 에서 호출 (fan_in >= 5), SPEC-AX-AUTH-001
// @MX:ANCHOR: [AUTO] Authorize — RBAC 결정 단일 진입점
// @MX:REASON: gRPC interceptor / REST middleware / FastAPI dep / RBAC 테스트 에서 호출 (fan_in >= 4)
package auth
