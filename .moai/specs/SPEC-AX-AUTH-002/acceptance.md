# SPEC-AX-AUTH-002 Acceptance Criteria

> Format: Given / When / Then
> Methodology: DDD or TDD per quality.yaml (AUTH-001과 동일 모드 유지)
> Tooling (Go): testify/assert + miniredis + bufconn + httptest + goleak + testcontainers-go

본 문서는 SPEC-AX-AUTH-002의 5개 REQ(1 Ubiquitous with 3 sub-clauses + 4 modal)에 대한 acceptance criteria를 정의한다. 모호한 표현 사용 금지. lessons #2 적용: REQ-AUTH2-UBI-001의 3 sub-clause(a/b/c)에 dedicated AC 각각 부여.

---

## §0. REQ-AUTH2-UBI-001 Transverse Invariant — Acceptance

본 섹션은 §1-§4 modal REQ를 가로지르는 UBI 불변 조건을 검증한다. 3 sub-clause(a: audit completeness, b: AuthEnabled=false skip, c: 사전 차단)에 각각 dedicated AC를 부여한다.

### AC-AUTH2-UBI-001-a (모든 권한 결정 audit — Deny path)

REQ 대응: REQ-AUTH2-UBI-001-a.

**Given**:
- AuthEnabled=true, validator 활성
- viewer 토큰(scope=`iroum-ax:viewer`, sub=`uuid-viewer-001`) 발급
- PostgreSQL `workflows` + `audit_logs` 테이블 clean state

**When**:
- 클라이언트가 `POST /api/v1/workflows` with viewer Bearer token + body `{"document_id":"d-uuid-ubi-a"}`

**Then**:
- HTTP 403 Forbidden
- `audit_logs` 테이블에 정확히 1 row INSERT: `action=AUTH_FORBIDDEN`, `user_id='uuid-viewer-001'`, `details.method='POST'`, `details.path='/api/v1/workflows'`, `details.required='write:workflow'`, `details.granted_roles=["viewer"]`
- `workflows` 테이블 변화 0건 (핸들러 미실행, 사전 차단)
- 동일 transaction 내 atomicity 유지 (audit row가 commit되었다면 비즈니스 부작용 없음 invariant)

### AC-AUTH2-UBI-001-a2 (Grant path — 별도 audit row 미생성)

REQ 대응: REQ-AUTH2-UBI-001-a (grant 부분).

**Given**: AuthEnabled=true, analyst 토큰(sub=`uuid-analyst-001`)
**When**: `POST /api/v1/workflows` body `{"document_id":"d-uuid-ubi-a2"}` (정상 권한 보유)
**Then**:
- HTTP 201 Created
- `audit_logs`에 `WORKFLOW_CREATED` row 1건 (핸들러가 작성), `user_id='uuid-analyst-001'`
- `audit_logs`에 `AUTH_FORBIDDEN` 또는 별도 `AUTH_GRANTED` row 0건 (grant는 row 미생성)
- 단, `WORKFLOW_CREATED` row의 `details.granted_permission='write:workflow'` 필드 존재 (context annotation 결과)

### AC-AUTH2-UBI-001-b (AuthEnabled=false 권한 체크 skip — 백워드 호환)

REQ 대응: REQ-AUTH2-UBI-001-b. 본 AC는 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-AUTH-001 회귀 invariant의 핵심.

**Given**:
- AuthEnabled=false (validator=nil, sandbox)
- 클라이언트가 `Authorization` 헤더 없이 요청

**When**: `POST /api/v1/workflows` body `{"document_id":"d-uuid-ubi-b"}` (no auth header)

**Then**:
- HTTP 201 Created (인증/인가 모두 우회)
- `workflows.user_id='cli-anonymous'`
- `audit_logs`의 `WORKFLOW_CREATED` row: `user_id='cli-anonymous'`
- `audit_logs`에 `AUTH_FORBIDDEN` row 0건
- SPEC-AX-CTRL-001 AC-CTRL-UBI-002-C + SPEC-AX-AUTH-001 AC-AUTH-UBI-001-C 결과와 byte-identical

### AC-AUTH2-UBI-001-c (사전 차단 — 핸들러 진입 전)

REQ 대응: REQ-AUTH2-UBI-001-c. 비즈니스 부작용 차단의 핵심 검증.

**Given**:
- AuthEnabled=true, viewer 토큰
- PostgreSQL `workflows`/`audit_logs` clean state, 행 수 측정 `beforeWorkflows=0`, `beforeAuditCreated=0`

**When**: `POST /api/v1/workflows` body `{"document_id":"d-uuid-ubi-c"}` with viewer token

**Then**:
- HTTP 403
- `workflows` count 변화 없음 (`afterWorkflows == beforeWorkflows`)
- `audit_logs` `WORKFLOW_CREATED` row 변화 없음 (`afterAuditCreated == beforeAuditCreated`)
- `audit_logs` `AUTH_FORBIDDEN` row 정확히 +1
- 핸들러 진입 후 차단이라면 `workflows` row가 PENDING 상태로 1건 생성되었을 것 → 미생성으로 사전 차단 invariant 검증

---

## §1. REQ-AUTH2-001 Method-Permission Mapping — Acceptance

### AC-AUTH2-001-1 (REST mapping positive — 8개 경로)

REQ 대응: REQ-AUTH2-001-E1.

**Given**: AuthEnabled=true, admin 토큰(모든 권한 보유)

**When**: 각 매핑 경로에 admin으로 요청
- `POST /api/v1/workflows` → write:workflow
- `GET /api/v1/workflows/{id}` → read:workflow
- `GET /api/v1/workflows` → read:workflow
- `DELETE /api/v1/workflows/{id}` → delete:workflow (RBAC 통과만 검증; 핸들러는 후속 SPEC `SPEC-AX-WF-DELETE-001` — D3 iter-2 fix)
- `POST /api/v1/recommendations/{id}/feedback` → write:recommendation
- `POST /api/v1/documents/upload` → write:workflow
- `GET /health` → bypass

**Then**: 모두 RBAC 단 통과(403 아님). 핸들러 응답 코드는 비즈니스 로직에 따름 (DELETE는 핸들러 부재로 501/404 가능, 단 403은 아님). admin DELETE 통과 검증의 상세 동작(workflow 삭제 + WORKFLOW_DELETED audit)은 본 SPEC 범위 외 — 후속 SPEC `SPEC-AX-WF-DELETE-001`. `/metrics`는 본 SPEC 범위 외(v0.1.2 Option C 분리, spec.md §5 Exclusion #13) — 후속 SPEC `SPEC-AX-OBS-001`/`SPEC-AX-METRICS-001`.

### AC-AUTH2-001-2 (REST mapping — Unknown path default-deny)

REQ 대응: REQ-AUTH2-001-U1.

**Given**: AuthEnabled=true, admin 토큰
**When**: `POST /api/v1/unknown-endpoint` (매핑 테이블에 없는 경로)
**Then**:
- HTTP 503 Service Unavailable
- 응답 body `{"error":{"code":"AUTHZ_MAPPING_MISSING","message":"authorization mapping not defined for this method"}}`
- `audit_logs`에 `AUTH_FORBIDDEN` row 1건, `details.reason='authz_mapping_missing'`, `details.method='POST'`, `details.path='/api/v1/unknown-endpoint'`
- 핸들러 미실행

### AC-AUTH2-001-3 (REST mapping — HEAD/OPTIONS bypass)

REQ 대응: REQ-AUTH2-001-E1 (HEAD/OPTIONS).

**Given**: AuthEnabled=true (토큰 유무 무관)
**When (case A)**: `OPTIONS /api/v1/workflows` with no Authorization header (CORS preflight)
**Then (case A)**:
- HTTP 200 또는 204 (핸들러가 CORS 응답을 생성하거나 default 처리)
- 인증/인가 모두 bypass
- audit_logs 변화 없음

**When (case B)**: `HEAD /api/v1/workflows`
**Then (case B)**: 동일 (bypass)

### AC-AUTH2-001-4 (gRPC mapping positive — 3 RPCs)

REQ 대응: REQ-AUTH2-001-E2.

**Given**: bufconn gRPC server with auth + authz interceptors, admin 토큰
**When**: 각 RPC 호출
- `WorkflowService.CreateWorkflow` → write:workflow
- `WorkflowService.GetWorkflow` → read:workflow
- `WorkflowService.ListWorkflows` → read:workflow

**Then**: 모두 RBAC 통과 (정상 응답 또는 비즈니스 에러; `codes.PermissionDenied` 아님).

### AC-AUTH2-001-5 (gRPC mapping — Unknown method default-deny)

REQ 대응: REQ-AUTH2-001-U1 (gRPC 측).

**Given**: bufconn, admin 토큰
**When**: 가상의 `WorkflowService.UnknownRPC` 호출 (proto 미정의 메서드 — info.FullMethod 임의 주입 mock)
**Then**:
- gRPC code `codes.Unavailable`
- 에러 메시지에 `authorization mapping not defined` 포함
- audit `AUTH_FORBIDDEN` reason=`authz_mapping_missing`

### AC-AUTH2-001-6 (gRPC Health bypass)

REQ 대응: REQ-AUTH2-003-S1.

**Given**: bufconn (토큰 없음)
**When**: `/grpc.health.v1.Health/Check` 호출 with no metadata
**Then**: 정상 응답 (인증 + 인가 모두 bypass). 다른 메서드 호출 시 `UNAUTHENTICATED` 반환 확인 (대조군).

### AC-AUTH2-001-Performance (Mapping lookup p99 < 100µs)

**Given**: 매핑 테이블 populated
**When**: `resolveRESTPermission` / `resolveGRPCPermission` 각 10000회 호출
**Then**:
- p99 < 100µs (`go test -bench=BenchmarkAuthzMapping`)
- p50 < 30µs
- CI에서는 1.5× 완화 (150µs / 45µs) 허용

---

## §2. REQ-AUTH2-002 REST Authorize Wrapper — Acceptance

### AC-AUTH2-002-1 (analyst POST → 201)

REQ 대응: REQ-AUTH2-002-E1.

**Given**: AuthEnabled=true, analyst 토큰(sub=`uuid-analyst-002`)
**When**: `POST /api/v1/workflows` body `{"document_id":"d-uuid-002-1"}`
**Then**:
- HTTP 201 Created
- `workflows` row 1건 INSERT (`user_id='uuid-analyst-002'`)
- `audit_logs` `WORKFLOW_CREATED` row, `details.granted_permission='write:workflow'`
- `audit_logs` `AUTH_FORBIDDEN` row 0건

### AC-AUTH2-002-2 (viewer POST → 403)

REQ 대응: REQ-AUTH2-002-U1.

**Given**: AuthEnabled=true, viewer 토큰(sub=`uuid-viewer-002`)
**When**: `POST /api/v1/workflows` body `{"document_id":"d-uuid-002-2"}`
**Then**:
- HTTP 403 Forbidden
- 응답 헤더 `WWW-Authenticate: Bearer realm="iroum-ax", error="insufficient_scope", scope="write:workflow"`
- 응답 body `{"error":{"code":"PERMISSION_DENIED","message":"insufficient scope","details":{"required":"write:workflow","granted":["viewer"]}}}`
- `audit_logs` `AUTH_FORBIDDEN` row 1건 (AC-AUTH2-UBI-001-a 동일 단언)
- `workflows` 변화 0건

### AC-AUTH2-002-3 (viewer GET → 200)

REQ 대응: REQ-AUTH2-002-E1 (read 권한 보유 경로).

**Given**: AuthEnabled=true, viewer 토큰; `workflows` 테이블에 fixture row 1건
**When**: `GET /api/v1/workflows/{fixture_id}`
**Then**:
- HTTP 200 OK
- 응답 body에 workflow JSON
- `audit_logs` `AUTH_FORBIDDEN` row 0건

### AC-AUTH2-002-4 (Missing user context — defense in depth)

REQ 대응: REQ-AUTH2-002-U2.

**Given**: AuthEnabled=true (validator non-nil), 그러나 테스트가 `auth.RESTMiddleware`를 우회하여 직접 `RESTAuthzMiddleware`만 chain (wiring 버그 시뮬레이션)
**When**: `POST /api/v1/workflows` (user context 미주입)
**Then**:
- HTTP 500 Internal Server Error
- 응답 body `{"error":{"code":"AUTHZ_USER_MISSING","message":"authenticated user context not propagated"}}`
- `audit_logs` `AUTH_FORBIDDEN` row reason=`user_context_missing`

### AC-AUTH2-002-5 (Multiple roles union — analyst+viewer)

REQ 대응: REQ-AUTH2-002-E1 + AUTH-001 REQ-AUTH-004-S1 union.

**Given**: AuthEnabled=true, 토큰 scope=`iroum-ax:analyst iroum-ax:viewer` (공백 구분 두 역할)
**When**: `POST /api/v1/workflows`
**Then**: HTTP 201 (analyst 권한으로 통과; union 결과 = analyst ∪ viewer)

### AC-AUTH2-002-6 (Unknown scope — silently dropped, default to no permission)

REQ 대응: REQ-AUTH2-002-U1 + AUTH-001 AC-AUTH-004-5.

**Given**: 토큰 scope=`iroum-ax:hacker iroum-ax:superuser` (allow-list 외)
**When**: `POST /api/v1/workflows`
**Then**:
- HTTP 403
- `details.granted_roles=[]` (unknown scope silently dropped)
- `audit_logs.AUTH_FORBIDDEN.details.reason='insufficient_permission'`

---

## §3. REQ-AUTH2-003 gRPC Authorize Interceptor — Acceptance

### AC-AUTH2-003-1 (analyst CreateWorkflow → OK)

REQ 대응: REQ-AUTH2-003-E1.

**Given**: bufconn, analyst 토큰 metadata
**When**: `WorkflowService.CreateWorkflow{document_id:"d-uuid-003-1"}`
**Then**:
- 정상 응답 (CreateWorkflowResponse)
- `workflows` 1건 INSERT (`user_id=token sub`)
- `audit_logs` `WORKFLOW_CREATED` row, no `AUTH_FORBIDDEN`

### AC-AUTH2-003-2 (viewer CreateWorkflow → PermissionDenied)

REQ 대응: REQ-AUTH2-003-U1.

**Given**: bufconn, viewer 토큰
**When**: `WorkflowService.CreateWorkflow{document_id:"d-uuid-003-2"}`
**Then**:
- gRPC code `codes.PermissionDenied`
- 에러 메시지에 `insufficient scope: required=write:workflow granted=[viewer]` 포함
- `audit_logs` `AUTH_FORBIDDEN` row 1건, `details.method='/iroum.ax.v1.WorkflowService/CreateWorkflow'`
- `workflows` 변화 0건

### AC-AUTH2-003-3 (viewer GetWorkflow → OK)

REQ 대응: REQ-AUTH2-003-E1 (read 권한 보유).

**Given**: bufconn, viewer 토큰, fixture workflow 존재
**When**: `WorkflowService.GetWorkflow{id: "<fixture_id>"}`
**Then**: 정상 응답, no `AUTH_FORBIDDEN`

### AC-AUTH2-003-4 (Health check bypass)

REQ 대응: REQ-AUTH2-003-S1.

**Given**: bufconn (메타데이터 없음)
**When**: `/grpc.health.v1.Health/Check`
**Then**: 정상 응답 (인증 + 인가 모두 bypass)

### AC-AUTH2-003-5 (AuthEnabled=false → cli-anonymous fallback)

REQ 대응: REQ-AUTH2-UBI-001-b (gRPC 경로 검증).

**Given**: bufconn, validator=nil, AuthEnabled=false
**When**: `WorkflowService.CreateWorkflow{document_id:"d-uuid-003-5"}` with no metadata
**Then**:
- 정상 응답 (인증/인가 bypass)
- `workflows.user_id='cli-anonymous'`
- `audit_logs` `WORKFLOW_CREATED` row, `user_id='cli-anonymous'`
- no `AUTH_FORBIDDEN`

### AC-AUTH2-003-6 (Interceptor chain order — auth before authz)

REQ 대응: REQ-AUTH2-003-E1 (chain order).

**Given**: bufconn server는 `grpc.ChainUnaryInterceptor(auth.UnaryServerInterceptor(...), server.UnaryAuthzInterceptor(...))` 순서 등록
**When**: 잘못된 서명 토큰 metadata로 `CreateWorkflow` 호출
**Then**:
- gRPC code `codes.Unauthenticated` (auth 단에서 차단)
- `codes.PermissionDenied` 아님 (authz 단까지 도달 안 함)
- `audit_logs` `AUTH_REJECTED` row 1건 (AUTH-001 처리), `AUTH_FORBIDDEN` 0건

---

## §4. REQ-AUTH2-004 E2E Forbidden Verification — Acceptance (AUTH-001 SKIP Unblock)

### AC-AUTH2-004-1 (viewer DELETE → 403 + AUTH-001 SKIP unblock)

REQ 대응: REQ-AUTH2-004-E1 + REQ-AUTH2-004-U1.

**Given**:
- testcontainers PostgreSQL + Redis + 인메모리 RSA JWT 키
- AuthEnabled=true, mockJWKSProvider (AUTH-001 `auth_e2e_test.go` 패턴 재사용)
- viewer 토큰(sub=`kepco-viewer-001`, scope=`iroum-ax:viewer`)
- 기존 `auth_e2e_test.go` `TestE2E_Auth_RBACForbidden` 함수의 `t.Skip(...)` 라인 제거 (S3 deliverable)
- fixture workflow `wf-fixture-004-1` 사전 INSERT (analyst 토큰으로 미리 생성)

**When**: `DELETE /api/v1/workflows/wf-fixture-004-1` with viewer Bearer token

**Then**:
- HTTP 403 Forbidden
- 응답 헤더 `WWW-Authenticate: Bearer realm="iroum-ax", error="insufficient_scope", scope="delete:workflow"`
- `audit_logs` `AUTH_FORBIDDEN` row 1건, `user_id='kepco-viewer-001'`, `details.required='delete:workflow'`, `details.granted_roles=["viewer"]`, `details.method='DELETE'`, `details.path='/api/v1/workflows/wf-fixture-004-1'`
- `workflows` 테이블에서 `wf-fixture-004-1` 여전히 존재 (삭제 안 됨)
- 본 시나리오는 AUTH-001 acceptance.md §6 AC-AUTH-E2E-3의 SKIP을 unblock

### AC-AUTH2-004-2 (viewer GET → 200, default-deny 비적용 검증 — D3 iter-2 fix)

REQ 대응: REQ-AUTH2-004-E2.

**Given**: 동일 인프라, viewer 토큰(sub=`kepco-viewer-002`); `workflows` 테이블에 fixture row `wf-fixture-004-2` 사전 INSERT

**When**: `GET /api/v1/workflows` with viewer Bearer token

**Then**:
- HTTP 200 OK
- 응답 body에 workflows JSON array (viewer가 read:workflow 권한 보유)
- `audit_logs` `AUTH_FORBIDDEN` row 0건 (viewer가 read endpoint에서 mapping-missing 503 fallback 없이 정상 통과)
- 본 AC는 viewer가 자신의 권한 범위 내 endpoint에서 default-deny 503 또는 403이 발생하지 않음을 검증 (positive case로 default-deny가 read 권한을 잘못 차단하지 않음을 보장)

### AC-AUTH2-004-Sprint7-Unblock (SKIP 마커 mechanical 제거 — D5 iter-2 fix)

REQ 대응: REQ-AUTH2-004-U1.

**Given**: S1 + S2 GREEN 종료, S3 진입

**When**: S3 deliverable 완료 후 `auth_e2e_test.go` 정적 점검

**Then**:
- `grep -c "SPEC-AX-AUTH-002: RBAC REST handler wiring deferred" apps/control-plane/internal/server/auth_e2e_test.go` 결과 = 0
- `auth_e2e_test.go` 내 `TestE2E_Auth_RBACForbidden` 함수 body에서 `t.Skip(...)` 호출 0건 (정규식 `grep -A 5 "func TestE2E_Auth_RBACForbidden" auth_e2e_test.go | grep -c "t.Skip"` = 0)
- `TestE2E_Auth_RBACForbidden` 실행 시 GREEN (AC-AUTH2-004-1 + AC-AUTH2-004-2 시나리오 모두 실제 실행)
- AUTH-001 acceptance.md §6 AC-AUTH-E2E-3 status가 `SKIP → ACTIVE (by SPEC-AX-AUTH-002 S3)` 마커로 업데이트됨

> AC-AUTH2-Metrics-Admin removed in v0.1.2 (Option C — spec.md §5 Exclusion #13). `/metrics` 권한 매핑과 핸들러 등록은 후속 SPEC `SPEC-AX-OBS-001` 또는 `SPEC-AX-METRICS-001`로 분리. 임시 운영 보호는 K8s NetworkPolicy / Docker network isolation / Helm values 차원으로 처리.

### AC-AUTH2-004-4 (Concurrent RBAC requests — 5 goroutine isolation)

REQ 대응: REQ-AUTH2-002 + REQ-AUTH2-003 동시성 격리.

**Given**:
- AuthEnabled=true
- 5개 서로 다른 토큰: 2× analyst, 2× viewer, 1× admin
- 각 goroutine이 서로 다른 endpoint 호출

**When**: 5개 goroutine 동시 실행
- analyst-1: `POST /api/v1/workflows` (예상 201)
- analyst-2: `GET /api/v1/workflows` (예상 200)
- viewer-1: `POST /api/v1/workflows` (예상 403)
- viewer-2: `GET /api/v1/workflows` (예상 200)
- admin-1: `GET /api/v1/workflows` (예상 200)

**Then**:
- 각 응답이 예상대로 (2 success + 1 forbidden + 2 reads OK)
- `audit_logs` `AUTH_FORBIDDEN` 정확히 1건 (`user_id` = viewer-1 sub)
- `audit_logs` `WORKFLOW_CREATED` 정확히 1건 (`user_id` = analyst-1 sub)
- `-race` 플래그 통과 (race condition 없음)
- `goleak.VerifyNone(t)` 통과

---

## §5. Performance Acceptance Summary

| Metric | Target | Measurement Method | Reference |
|--------|--------|--------------------|-----------|
| Mapping lookup p99 | < 100µs | `go test -bench=BenchmarkAuthzMapping` | AC-AUTH2-001-Performance |
| Authorize wrapper end-to-end p99 | < 1ms | `go test -bench=BenchmarkAuthzMiddleware` | spec.md §4 NFR (AUTH-001 재사용) |
| Default-deny response | < 5ms | `go test -bench=BenchmarkDefaultDeny` | spec.md §4 |
| Backward compat regression | 0 AC failure in SPEC-AX-AUTH-001 + SPEC-AX-CTRL-001 with AuthEnabled=false | E2E run | AC-AUTH2-UBI-001-b |
| Concurrent isolation | 5 goroutines, race-free | `go test -race` | AC-AUTH2-004-4 |
| AUTH-001 SKIP unblock | TestE2E_Auth_RBACForbidden GREEN | E2E run | AC-AUTH2-004-1 |

---

## §6. Edge Case Catalog

| Edge Case | 대응 AC | 비고 |
|-----------|--------|------|
| 매핑 미정의 method/path → default-deny 503 | AC-AUTH2-001-2 / AC-AUTH2-001-5 | REQ-AUTH2-001-U1 핵심 |
| AuthEnabled=false 시 권한 체크 skip | AC-AUTH2-UBI-001-b / AC-AUTH2-003-5 | 백워드 호환 invariant |
| 다중 role union (analyst+viewer) → 가장 강한 권한 적용 | AC-AUTH2-002-5 | AUTH-001 union 결과 신뢰 |
| HEAD/OPTIONS HTTP 메서드 (CORS preflight) | AC-AUTH2-001-3 | bypass |
| Unknown scope (`iroum-ax:hacker`) → silently dropped | AC-AUTH2-002-6 | AUTH-001 AC-AUTH-004-5 패턴 |
| User context 미주입 (middleware wiring 버그) | AC-AUTH2-002-4 | defense in depth |
| 핸들러 진입 후 차단 (사전 차단 invariant 위반) | AC-AUTH2-UBI-001-c | 부작용 차단 |
| Interceptor 순서 오류 (authz 먼저) | AC-AUTH2-003-6 + `TestBuildGRPCInterceptorChain_Order` | chain order 검증 (D7 iter-2) |
| REST 미들웨어 순서 오류 | `TestBuildRESTChain_Order` | chain order 검증 (D7 iter-2) |
| 동시성 race condition (5 goroutine) | AC-AUTH2-004-4 | `-race` + goleak |
| viewer GET → 200 default-deny 비적용 검증 | AC-AUTH2-004-2 (D3 iter-2) | viewer read 권한 통과 |
| AUTH-001 SKIP marker mechanical 제거 | AC-AUTH2-004-Sprint7-Unblock (D5 iter-2) | grep 단언 |
| In-flight 권한 변경 race (Authorize 통과 후 role 변경) | Exclusion §10 #10 (D6 iter-2) | JWT immutable + 만료 1h로 자연 해소 |
| AUTH-001 백워드 호환 회귀 | AC-AUTH2-UBI-001-b + AC-AUTH2-003-5 | 결정적 검증점 |

---

## §7. Definition of Done (Acceptance Phase)

본 SPEC의 acceptance가 완료되었다고 선언하기 위해 모두 PASS 필요:

- [ ] §0: REQ-AUTH2-UBI-001 dedicated AC 4개(UBI-001-a/a2/b/c) 자동화 테스트 통과
- [ ] §1: REQ-AUTH2-001 AC 6개 + performance benchmark 1개
- [ ] §2: REQ-AUTH2-002 AC 6개
- [ ] §3: REQ-AUTH2-003 AC 6개
- [ ] §4: REQ-AUTH2-004 E2E AC 4개 (viewer DELETE + viewer GET default-deny 비적용 + 동시성 + Sprint7-Unblock SKIP 제거) — v0.1.2에서 AC-AUTH2-Metrics-Admin 삭제(spec.md §5 Exclusion #13, Option C 분리)
- [ ] §5: 5개 성능 지표 모두 target 충족 (CI에서는 1.5× 완화 허용)
- [ ] §6: 13개 edge case 모두 대응 AC로 검증됨 (D2/D3/D5/D6/D7 iter-2 fix + v0.1.2 Option C 반영)
- [ ] coverage ≥ 85% (`go test -cover ./apps/control-plane/internal/server/...`)
- [ ] golangci-lint default 0 issue
- [ ] `goleak.VerifyNone(t)` 모든 Go 테스트 통과
- [ ] `go test -race ./...` 통과
- [ ] AUTH-001 `auth_e2e_test.go` `TestE2E_Auth_RBACForbidden` SKIP 제거 + GREEN (AC-AUTH2-004-Sprint7-Unblock grep 단언 PASS)
- [ ] AUTH-001 acceptance.md §6 AC-AUTH-E2E-3 status `SKIP → ACTIVE (by SPEC-AX-AUTH-002 S3)` 마커 추가
- [ ] MX tag 등록: `authz.go`에 ANCHOR 1개, NOTE 1개, WARN 1개 + `chain.go`에 ANCHOR 1개, NOTE 1개 + 모두 REASON 보유
- [ ] D7 chain order 단위 테스트 (`TestBuildRESTChain_Order` + `TestBuildGRPCInterceptorChain_Order`) GREEN
- [ ] manager-quality TRUST 5 통과
- [ ] evaluator-active per-sprint scoring: S0 ≥ 0.80 (foundation security-critical), S1/S2 ≥ 0.80, S3 ≥ 0.75
- [ ] AuthEnabled=false 백워드 호환 regression 0건 (SPEC-AX-CTRL-001 + SPEC-AX-AUTH-001 모든 AC unchanged 통과)

**Total AC count**: 22 (§0: 4, §1: 6 + 1 perf, §2: 6, §3: 6, §4: 4 — viewer DELETE / viewer GET (D3 redefined) / 동시성 / Sprint7-Unblock (D5)) — 단위/통합/E2E 골고루 분포. **v0.1.2 (iter 3) 변경**: AC-AUTH2-Metrics-Admin (3 sub-cases) 삭제로 25 → 22. /metrics는 Option C로 후속 SPEC `SPEC-AX-OBS-001`/`SPEC-AX-METRICS-001`에 분리(spec.md §5 Exclusion #13). iter 2 변경: AC-AUTH2-004-3 (admin DELETE) 삭제, AC-AUTH2-004-2 viewer GET로 재정의.
