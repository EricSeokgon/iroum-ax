# SPEC-AX-AUTH-002 진행 현황

## Sprint 이력

### Sprint 1 (기완료)
- 상태: DONE
- 범위: JWT 인증 미들웨어 (RESTMiddleware, UnaryServerInterceptor)
- 주요 구현: TokenValidator, JWKS provider, authn chain

### Sprint 2 (기완료)
- 상태: DONE
- 범위: RBAC 인가 미들웨어 (RESTAuthzMiddleware, UnaryAuthzInterceptor)
- 주요 구현: authz mapping, permission check, 감사 로그
- 참조: .moai/sprints/SPEC-AX-AUTH-002/

### Sprint 3 — E2E Integration + AUTH-001 SKIP Unblock (완료)
- 상태: DONE
- 날짜: 2026-05-15
- Sprint 계약: .moai/sprints/SPEC-AX-AUTH-002/sprint-S3-E2E.md

#### AC 달성 현황

| AC | 설명 | 상태 |
|---|---|---|
| AC-AUTH2-004-1 | admin 토큰 → POST/GET workflows → 201/200 | PASS |
| AC-AUTH2-004-2 | viewer POST → 403, gRPC CreateWorkflow → PermissionDenied | PASS |
| AC-AUTH2-004-4 | analyst POST → 201, GET → 200 | PASS |
| AC-AUTH2-004-Sprint7-Unblock | TestE2E_Auth_RBACForbidden SKIP 제거 + viewer DELETE → 403 | PASS |

#### 생성/수정 파일

| 파일 | 변경 유형 | 설명 |
|---|---|---|
| apps/control-plane/internal/server/authz_e2e_test.go | NEW | 5개 E2E 통합 테스트 |
| apps/control-plane/internal/server/auth_e2e_test.go | MODIFIED | TestE2E_Auth_RBACForbidden SKIP 제거 |
| .moai/sprints/SPEC-AX-AUTH-002/sprint-S3-E2E.md | NEW | Sprint 계약서 |

#### 검증 결과

```
go build -tags=integration ./apps/control-plane/...: PASS
go vet -tags=integration ./apps/control-plane/...:   PASS
golangci-lint --build-tags=integration:               PASS
go test (unit) ./apps/control-plane/internal/...:    all PASS (cached)
go test -tags=integration -run="TestE2E_Auth":
  - TestE2E_Authz_AdminFullAccess:            PASS (9.12s)
  - TestE2E_Authz_ViewerForbidden_POST:       PASS (9.07s)
  - TestE2E_Authz_AnalystWriteAllowed:        PASS (9.61s)
  - TestE2E_Authz_AuthDisabled_BypassesAuthz: PASS (7.40s)
  - TestE2E_GRPC_Authz_ViewerForbidden_Create: PASS
  - TestE2E_Auth_RBACForbidden:               PASS (SKIP 제거 확인)
grep -c "t.Skip" auth_e2e_test.go: 0
```

## 전체 SPEC 상태

- 상태: **COMPLETE**
- 모든 스프린트 완료
- 모든 AC 달성
- 단위 테스트 + E2E 통합 테스트 모두 PASS
