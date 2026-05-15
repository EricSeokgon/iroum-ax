# SPEC-AX-AUTH-002 Plan — RBAC REST/gRPC Handler 통합

> Author: manager-spec (Opus 4.7)
> Methodology: DDD or TDD per `.moai/config/sections/quality.yaml` development_mode (AUTH-001과 동일 모드 유지)
> Harness Level: thorough (security-critical SPEC; AUTH-001과 동일)
> Token budget: ~800K (4 sprints, in-process wiring 중심으로 AUTH-001 1.2M보다 작음 — lessons #6 적용)

본 문서는 SPEC-AX-AUTH-002 spec.md의 5개 REQ 모듈을 4개 Sprint로 분해한다. 각 Sprint는 진입 조건(Entry), 산출물(Deliverable), 종료 조건(Exit), 위험(Risk)을 명시한다.

---

## 0. Sprint DAG

```
S0 (Foundation: 매핑 테이블 + 단위 테스트) — sequential prerequisite
   ↓
S1 (REST RESTAuthzMiddleware)  ─┐
                                  ├──→ S3 (E2E AUTH-001 SKIP unblock)
S2 (gRPC UnaryAuthzInterceptor) ─┘
```

- S0 → S1 + S2 (병렬 가능) → S3 (순차)
- S1과 S2는 file ownership 분리(REST vs gRPC interceptor 등록)로 worktree isolation 가능 시 병렬, 단일 세션에서는 S1 → S2 순차 권장

---

## 1. Sprint S0 — Foundation: 매핑 테이블 + Default-Deny Safety Net

### Entry
- AUTH-001 v0.1.1 GREEN 상태 확인 (`internal/auth/rbac.go` exports: `Authorize`, `LogForbidden`, `ParseRolesFromScope`, `EffectivePermissions`)
- `internal/audit/actions.go`에 `ActionAuthForbidden` 상수 존재 확인
- `internal/auth/errors.go`에 `ErrInsufficientPermission` 존재 확인

### Deliverable
- `apps/control-plane/internal/server/authz.go` 신규 — 다음 구성요소:
  - `type permissionMap struct { method, path string }` (REST 키)
  - `restPermissionTable map[permissionMap]string` (canonical REST 매핑, REQ-AUTH2-001-E1 매트릭스 그대로)
  - `grpcPermissionTable map[string]string` (gRPC FullMethod → Permission)
  - `bypassPaths map[permissionMap]bool` (`/health`, `/metrics`, HEAD/OPTIONS)
  - `bypassGRPCMethods map[string]bool` (`/grpc.health.v1.Health/Check`)
  - `func resolveRESTPermission(method, path string) (perm string, bypass bool, mapped bool)` — `mapped=false` 시 default-deny 트리거
  - `func resolveGRPCPermission(fullMethod string) (perm string, bypass bool, mapped bool)`
  - `// @MX:ANCHOR: [AUTO] REST/gRPC 권한 매핑 단일 진입점 (fan_in >= 4)`
  - `// @MX:REASON: REQ-AUTH2-001-U1 default-deny 보안 결정의 immutable source-of-truth`
- `apps/control-plane/internal/server/authz_test.go` 신규 — 테이블 드리븐 단위 테스트:
  - REST 매핑 positive (8개 경로 × allowed roles)
  - REST 매핑 negative (unknown path → default-deny)
  - HEAD / OPTIONS bypass
  - gRPC 매핑 positive (3 RPCs)
  - gRPC 매핑 negative (unknown method → default-deny)
  - lookup performance benchmark `BenchmarkAuthzMapping` (target p99 < 100µs, 측정만; CI gate는 NFR 1.5× 완화)

### Exit
- `go test ./apps/control-plane/internal/server/... -run TestAuthzMapping -v` 모두 PASS
- `go test ./apps/control-plane/internal/server/... -bench=BenchmarkAuthzMapping` 측정 결과 plan.md 결과 섹션에 기록
- 매핑 테이블이 spec.md §3.2 REQ-AUTH2-001-E1/E2 매트릭스와 byte-identical 매칭 확인 (수작업 cross-check)
- `golangci-lint run ./apps/control-plane/internal/server/...` 0 issue
- MX tag 등록: `authz.go`에 ANCHOR 1개 + REASON 1개

### Risk
- **매핑 테이블 누락 위험**: 경로 추가 시 매핑 미동기. **Mitigation**: spec.md §3.2 매트릭스가 단일 source-of-truth, 향후 새 endpoint 추가 SPEC은 본 매트릭스 변경을 명시해야 함 (process guard).
- **테스트 fixture 마이그레이션**: rest_handler_test.go가 RBAC 없이 작성되어 있어 새 시나리오 추가 시 fixture 인증 토큰 발급 필요. **Mitigation**: S1에서 처리, S0는 매핑 lookup만 독립 테스트.

---

## 2. Sprint S1 — REST RESTAuthzMiddleware

### Entry
- S0 PASS
- `auth.RESTMiddleware` (AUTH-001 GREEN) 동작 확인

### Deliverable
- `authz.go`에 다음 추가:
  - `func RESTAuthzMiddleware(recorder auditRecorder) func(http.Handler) http.Handler` (decorator)
    - 진입 시 path/method 추출 → `resolveRESTPermission` → bypass면 next 호출 → mapped=false면 503 + audit reason=`authz_mapping_missing` → user 추출 후 `auth.Authorize(ctx, perm)` → 성공이면 ctx에 `granted_permission` annotation 추가 후 next, 실패면 403 + `LogForbidden`
  - `func writeForbidden(w http.ResponseWriter, required string, granted []auth.Role)` 헬퍼 (JSON body + WWW-Authenticate 헤더)
  - `func writeMappingMissing(w http.ResponseWriter)` (503 body)
  - `// @MX:NOTE: [AUTO] 인증 통과 직후 권한 결정의 단일 결정점 (REQ-AUTH2-UBI-001-c 사전 차단)`
  - default-deny 분기에 `// @MX:WARN: [AUTO] 매핑 미정의 시 503으로 fail-closed`
  - `// @MX:REASON: 보안 결정 누락 방지; 매핑 hot-patch 금지`
- `rest_handler.go` `Mux()` 수정 — 본 SPEC은 wiring만, 단 `Mux()` 자체는 인증/인가 미들웨어 외부에서 chain되므로 본 파일 변경은 최소. `cmd/server/server.go`에서 `auth.RESTMiddleware(validator)(server.RESTAuthzMiddleware(recorder)(handler.Mux()))` 순서로 chain.
- `cmd/server/server.go` 수정 — REST chain 등록 한 줄 + audit recorder 주입.
- `rest_handler_test.go` 추가 시나리오:
  - viewer POST `/api/v1/workflows` → 403 + audit_logs 1 row + workflows 변화 없음
  - analyst POST → 201 (정상)
  - admin POST → 201 (정상)
  - AuthEnabled=false (validator=nil) → 403 발생 안 함 (REQ-AUTH2-UBI-001-b 검증)
  - unknown path `/api/v1/unknown` → 503 + audit reason=`authz_mapping_missing`

### Exit
- 추가 4개 시나리오 모두 GREEN
- 기존 `rest_handler_test.go` 12개 + AUTH-001 기존 테스트 모두 regression 없이 PASS
- `auth_e2e_test.go` `TestE2E_Auth_FullChainWithValidToken` (AC-AUTH-E2E-1) + `TestE2E_Auth_AnonymousFallback` (AC-AUTH-E2E-2) regression 없이 PASS — 본 SPEC의 백워드 호환성 invariant 핵심

### Risk
- **AuthEnabled=false 경로 우회 실수**: `RESTAuthzMiddleware` 자체가 `auth.UserFromContext`로 user 추출 시 `ok=false`면 REQ-AUTH2-UBI-001-b(skip) vs REQ-AUTH2-002-U2(500) 분기 판단 필요. **Mitigation**: `RESTAuthzMiddleware` 생성자에 `authEnabled bool` 인자로 명시 주입 (validator nil 검사 대신 explicit flag) — `cmd/server/server.go`에서 wiring 시 `config.AuthEnabled` 전달. AuthEnabled=false 시 middleware 자체를 chain에서 제외하는 옵션도 검토(더 단순).
- **Audit row 중복 위험**: 핸들러가 자체 audit row 작성 시 grant 결정을 별도 row로 작성하면 중복. **Mitigation**: REQ-AUTH2-UBI-001-a 명시대로 grant는 context annotation만, deny만 별도 row.

---

## 3. Sprint S2 — gRPC UnaryAuthzInterceptor

### Entry
- S0 PASS (S1과 병렬 시작 가능)
- `auth.UnaryServerInterceptor` (AUTH-001 GREEN) 동작 확인

### Deliverable
- `authz.go`에 다음 추가:
  - `func UnaryAuthzInterceptor(recorder auditRecorder, authEnabled bool) grpc.UnaryServerInterceptor`
    - `info.FullMethod` 추출 → `resolveGRPCPermission` → bypass면 handler 호출 → mapped=false면 `codes.Unavailable` 반환 + audit → `auth.UserFromContext` → `auth.Authorize` → 실패 시 `codes.PermissionDenied` + `LogForbidden`
- `cmd/server/server.go` 수정 — gRPC chain 등록: `grpc.ChainUnaryInterceptor(auth.UnaryServerInterceptor(validator), server.UnaryAuthzInterceptor(recorder, cfg.AuthEnabled))`
- `grpc_server_test.go` 추가 시나리오:
  - viewer `CreateWorkflow` → `codes.PermissionDenied` + audit row + DB 변화 없음
  - analyst `CreateWorkflow` → OK (정상)
  - viewer `GetWorkflow` → OK (read:workflow 보유)
  - `/grpc.health.v1.Health/Check` bypass → 인증/인가 모두 skip
  - AuthEnabled=false → interceptor가 등록되었지만 user_id=cli-anonymous 폴백 (REQ-AUTH2-UBI-001-b)

### Exit
- 추가 5개 시나리오 모두 GREEN
- 기존 `grpc_server_test.go` + AUTH-001 gRPC 시나리오 regression 없이 PASS
- `cmd/server/server.go` 빌드 OK, server 기동 시 chain 순서 로그 출력 (`auth → authz → handler`)

### Risk
- **Interceptor 순서 오류**: `authz` 먼저 등록되면 user context 미주입 상태로 진입 → REQ-AUTH2-002-U2 trigger되어 500 무한 발생. **Mitigation**: `cmd/server/server.go`에 `grpc.ChainUnaryInterceptor(auth.UnaryServerInterceptor(...), server.UnaryAuthzInterceptor(...))` 순서를 코멘트와 함께 명시, `@MX:NOTE` 추가.
- **TestE2E_Auth_ConcurrentRequests regression**: AUTH-001 Sprint 7의 5-goroutine 동시성 테스트가 본 SPEC interceptor 추가로 race condition 노출 가능. **Mitigation**: S2 종료 시 `go test -race -run TestE2E_Auth_ConcurrentRequests` 통과 검증.

---

## 4. Sprint S3 — E2E AUTH-001 SKIP Unblock + Cross-SPEC Artifact

### Entry
- S1 + S2 모두 PASS
- `internal/server/authz.go` REST + gRPC wiring 안정화

### Deliverable
- `apps/control-plane/internal/server/auth_e2e_test.go` 수정:
  - `TestE2E_Auth_RBACForbidden` `t.Skip(...)` 라인 제거
  - 3 시나리오 활성화:
    - viewer 토큰 + `DELETE /api/v1/workflows/{id}` → HTTP 403 + audit_logs 1 row (`AUTH_FORBIDDEN` with `details.required=delete:workflow`, `details.granted_roles=["viewer"]`) + workflows row 미삭제 검증
    - analyst 토큰 + `GET /api/v1/workflows` → HTTP 200 + workflows list 반환
    - admin 토큰 + `DELETE /api/v1/workflows/{id}` → RBAC 단 통과(403 아님). DELETE 핸들러는 본 SPEC 범위 외이므로 501/404 허용. 핵심 assertion: response가 403이 아니라는 것 + audit `AUTH_FORBIDDEN` 미생성.
  - TODO 주석 제거 + `SPEC-AX-AUTH-002 RBAC wired` 마커 추가
- `schemas/openapi/openapi.yaml` 수정 — `403 Forbidden` 응답 스키마 + `WWW-Authenticate: Bearer ... error="insufficient_scope"` 헤더 명세 추가 (cross-SPEC documentation artifact).
- Plan.md 결과 섹션에 cross-SPEC artifact 변경 명시 (lessons #5 적용):
  - AUTH-001 `auth_e2e_test.go:354~371` SKIP 제거 (line numbers shift 가능, marker는 test name `TestE2E_Auth_RBACForbidden`)
  - AUTH-001 acceptance.md §6 AC-AUTH-E2E-3 status: `SKIP → ACTIVE (by SPEC-AX-AUTH-002 S3)`

### Exit
- `TestE2E_Auth_RBACForbidden` GREEN (3 시나리오 모두 PASS)
- AUTH-001 전체 E2E suite (`go test -tags=integration ./apps/control-plane/internal/server/... -v`) regression 없이 PASS
- `TestE2E_Auth_AnonymousFallback` (REQ-AUTH2-UBI-001-b 백워드 호환) regression 없이 PASS — 결정적 검증점
- coverage ≥ 85% 유지 (`go test -cover`)
- OpenAPI 스키마 `swagger-cli validate` 통과 (또는 동등 검증)
- manager-quality TRUST 5 통과
- evaluator-active per-sprint scoring ≥ 0.80 (security-critical wiring)

### Risk
- **DELETE 핸들러 부재로 admin 시나리오 모호성**: REQ-AUTH2-004-E3가 "RBAC 통과 검증"만 요구하나 핸들러가 501/404 반환 시 audit `WORKFLOW_DELETED`가 생성되지 않음. **Mitigation**: AC를 "403이 아니다" + "AUTH_FORBIDDEN row 미생성"으로 제한 (절대값 단언 회피 — lessons #4 stub-assert anti-pattern 회피 패턴 적용).
- **Cross-SPEC 골든 파일 영향**: AUTH-001 `celery_envelope_v2.json`은 본 SPEC에서 변경 없음 (Celery envelope 변경 없음). 영향 없음 확인.

---

## 5. 종합 위험 분석 및 Mitigation

| Risk ID | 영역 | Likelihood | Impact | Mitigation | Sprint 매핑 |
|---------|------|-----------|--------|-----------|------------|
| R-AUTH2-001 | 매핑 누락 default-deny safety net 미동작 | Low | Critical | S0 단위 테스트가 unknown path/method 명시 검증 + 503 응답 확정 | S0, S1 |
| R-AUTH2-002 | Interceptor chain 순서 오류 | Medium | High | `cmd/server/server.go`에 `@MX:NOTE` + 빌드 시 lint check + S2 통합 테스트 | S2 |
| R-AUTH2-003 | AuthEnabled=false regression (백워드 호환 깨짐) | Medium | High | `RESTAuthzMiddleware` 생성자에 `authEnabled` explicit flag; S1/S2 모두 AuthEnabled=false 시나리오 명시; `TestE2E_Auth_AnonymousFallback` regression 가드 | S1, S2, S3 |
| R-AUTH2-004 | Audit row 중복(grant + deny + handler audit) | Low | Medium | REQ-AUTH2-UBI-001-a로 grant는 context annotation만, deny만 별도 row 명시 | S1, S2 |
| R-AUTH2-005 | 핸들러 진입 후 권한 거부 (REQ-AUTH2-UBI-001-c 위반) | Low | Critical | Middleware/interceptor가 next 호출 전에 Authorize, S1/S2 테스트가 핸들러 실행 부작용(DB write) 부재 검증 | S1, S2, S3 |
| R-AUTH2-006 | TestE2E_Auth_ConcurrentRequests race | Low | Medium | S2 종료 시 `-race` 플래그 강제 + goleak.VerifyNone | S2, S3 |
| R-AUTH2-007 | DELETE 핸들러 부재로 E2E E3 시나리오 모호 | Medium | Low | AC 단언을 "403이 아니다" + "AUTH_FORBIDDEN 미생성"으로 제한 (절대값 회피) | S3 |
| R-AUTH2-008 | evaluator-active 보수성으로 score 0.10 편차 | Medium | Medium | thorough harness 적용, Security dimension 강화, plan-auditor 0.85 목표 (evaluator -0.10 buffer 후 ≥ 0.75) | 전 Sprint |

---

## 6. Cross-SPEC Impact (lessons #5 적용)

본 SPEC이 영향을 주는 다른 SPEC의 artifact:

| SPEC | Artifact | 변경 내용 | 변경 책임 |
|------|----------|----------|----------|
| SPEC-AX-AUTH-001 | `apps/control-plane/internal/server/auth_e2e_test.go` `TestE2E_Auth_RBACForbidden` | `t.Skip(...)` 제거 + 3 시나리오 활성화 | 본 SPEC S3 |
| SPEC-AX-AUTH-001 | `acceptance.md` §6 AC-AUTH-E2E-3 status | `SKIP → ACTIVE (by SPEC-AX-AUTH-002 S3)` 마커 추가 | 본 SPEC S3 (acceptance.md edit) |
| SPEC-AX-CTRL-001 | (없음) | 본 SPEC은 핸들러 비즈니스 로직 변경 없음. CTRL-001 모든 AC 영향 없음 | - |
| SPEC-AX-001 | (없음) | audit_logs schema 변경 없음. `cli-anonymous` 폴백 보존 | - |

본 SPEC 진행 중 SPEC-AX-AUTH-001 전체 test suite 통과 유지가 CI 의무. 회귀 발생 시 본 SPEC 차단.

---

## 7. Token Budget 예상 (lessons #6 적용)

| Sprint | RED + GREEN 추정 |
|--------|-----------------|
| S0 (매핑 + 단위 테스트) | ~100-150K |
| S1 (REST middleware) | ~200-250K |
| S2 (gRPC interceptor) | ~200-250K |
| S3 (E2E unblock + cross-SPEC) | ~150-200K |
| Sync (manager-quality + manager-docs) | ~100-150K |
| **Total** | **~750-1000K** |

본 SPEC은 in-process wiring 중심으로 AUTH-001(1.2M)보다 작은 예산 예상. 단일 세션 진행 가능. 단, evaluator-active iter 추가 시 50-100K 추가 가능.

---

## 8. Definition of Done (Plan Phase)

- [x] 5 REQ 모듈(UBI + 001~004)을 4 Sprint로 분해
- [x] 각 Sprint Entry/Deliverable/Exit/Risk 명시
- [x] Sprint DAG (S0 → {S1, S2} → S3) 명시
- [x] 8개 Risk + mitigation + sprint 매핑
- [x] Cross-SPEC artifact 변경 책임 명시 (lessons #5)
- [x] Token budget 추정 (lessons #6)
- [x] AUTH-001 E2E SKIP unblock 책임 S3에 명시
- [x] AuthEnabled=false 백워드 호환성 모든 Sprint에서 검증 명시
- [x] MX tag 등록 계획 (ANCHOR 1, NOTE 2, WARN 1)
