# SPEC-AX-AUTH-001 — Implementation Plan

> Phase: Plan
> Methodology: TDD (RED-GREEN-REFACTOR), thorough harness
> Sprint Structure: 7개 sub-sprint (S0~S6), 의존성 DAG 기반 순차 진행
> NO time estimates — priority-based ordering only (per `agent-common-protocol.md` §Time Estimation)

본 문서는 `spec.md`가 정의한 6개 REQ(UBI-001 + AUTH-001~005)를 TDD로 구현하기 위한 sprint 분해, 의존성, MX tag 계획, risk register를 담는다.

---

## 1. Sprint 분해 (Sprint Decomposition)

### S0 — Scaffolding (Priority: High)

**목표**: 신규 디렉토리 + 의존성 추가 + 공통 타입 정의

**산출물**:
- `apps/control-plane/internal/auth/` 디렉토리 생성 + 8개 stub 파일 (`@MX:TODO Sprint 1+`)
- `pipelines/auth/` 디렉토리 생성 + 7개 stub 파일
- `pkg/auth/claims.go` + `pkg/auth/claims.py` Claims 구조체 정의
- `apps/control-plane/internal/config/config.go`에 OIDC 필드 4개 추가 (OIDCIssuerURL, OIDCAudience, OIDCJWKSCacheTTL, RBACScopePrefix)
- `pipelines/config/settings.py`에 OIDC 필드 4개 추가
- go.mod / pyproject.toml에 의존성 추가:
  - Go: `github.com/golang-jwt/jwt/v5 v5.2.x`, `github.com/coreos/go-oidc/v3 v3.10.x`
  - Python: `python-jose[cryptography] ^3.3.0`, `httpx ^0.27.0`
- `tests/integration/test_auth_backward_compat.py` 골격 작성 (regression 가드, AuthEnabled=false 동작 보존)
- AuthEnabled 분기 wiring: 모든 신규 미들웨어가 AuthEnabled=false 시 no-op으로 동작하는 dispatch 골격

**Definition of Done**:
- 컴파일 + lint 통과 (비즈니스 로직 0줄)
- `test_auth_backward_compat.py`가 AuthEnabled=false에서 SPEC-AX-CTRL-001 AC-CTRL-UBI-002-A/B/C 동일 결과 반환 확인 (regression baseline)
- 모든 stub 파일에 `@MX:TODO` 태그 + Sprint 번호 명시

### S1 — JWT Verifier (Priority: High)

**목표**: REQ-AUTH-001 (JWT 검증) GREEN

**REQ 매핑**: REQ-AUTH-001-E1, REQ-AUTH-001-E2, REQ-AUTH-001-S1, REQ-AUTH-001-U1, REQ-AUTH-001-U2

**TDD 순서**:
- RED: `verifier_test.go` — 8개 테이블 케이스 (만료, 미래 iat, aud 불일치, alg 거부, 서명 실패, clock skew within, clock skew exceed, valid)
- GREEN: `verifier.go` — `golang-jwt/jwt/v5` 기반 검증, allow-list `[RS256, EdDSA, ES256]`
- RED: `jwks_cache_test.go` — fetch + 1h TTL + 키 회전 (kid 변경 시 즉시 refetch)
- GREEN: `jwks_cache.go` — `coreos/go-oidc/v3` JWKS 클라이언트 wrapping, in-memory sync.RWMutex 캐시
- RED: `blacklist_test.go` — miniredis 기반 SET/GET, TTL 검증, 동시성
- GREEN: `blacklist.go` — Redis SET with EXPIREAT = token.exp
- Python 평행 구현: `pipelines/auth/verifier.py` + `tests/test_verifier.py` (python-jose 또는 PyJWT)

**Definition of Done**:
- AC-AUTH-001-1~7 (acceptance.md §1) 통과
- coverage ≥ 85% (`verifier.go`, `jwks_cache.go`, `blacklist.go`)
- 토큰 검증 p99 < 5ms (JWKS cache hit) — benchmark

### S2 — OIDC Discovery (Priority: High)

**목표**: REQ-AUTH-002 (OIDC Provider Integration) GREEN

**REQ 매핑**: REQ-AUTH-002-E1, REQ-AUTH-002-O1

**TDD 순서**:
- RED: `oidc_client_test.go` — httptest 기반 mock `/.well-known/openid-configuration` 응답 파싱, timeout (10s) 검증, 실패 시 panic 검증
- GREEN: `oidc_client.go` — `coreos/go-oidc/v3` Provider 활용, startup-time 단발 fetch
- Python 평행 구현: `pipelines/auth/oidc_client.py` + `tests/test_oidc_client.py`

**Definition of Done**:
- AC-AUTH-002-1~3 통과
- Discovery 미응답 시 startup panic 확인 (fail-fast invariant)

### S3 — Middleware Integration (Priority: High)

**목표**: REQ-AUTH-003 (gRPC + REST + FastAPI 미들웨어) GREEN

**REQ 매핑**: REQ-AUTH-003-E1 (gRPC), REQ-AUTH-003-E2 (REST), REQ-AUTH-003-E3 (FastAPI), REQ-AUTH-003-U1 (missing header)

**TDD 순서**:
- RED: `grpc_interceptor_test.go` — bufconn 기반, Authorization metadata 추출, valid/expired/missing token 3 케이스, `/grpc.health.v1.Health/Check` bypass 검증
- GREEN: `grpc_interceptor.go` — `UnaryServerInterceptor` + `StreamServerInterceptor`, `auth.WithUser(ctx, user)` 주입
- RED: `rest_middleware_test.go` — httptest, `Authorization: Bearer <token>` 추출, `/health` bypass, `/api/v1/*` enforce
- GREEN: `rest_middleware.go` — `http.Handler` 체인, `WWW-Authenticate` 헤더 응답
- 기존 `grpc_server.go` + `rest_handler.go` 수정: hardcoded `"cli-anonymous"` 제거, `auth.UserFromContext(ctx).UID` 사용 (AuthEnabled=false 시는 빈 string → recorder가 cli-anonymous 폴백)
- 기존 `audit/recorder.go` 수정: `resolveUserID` 확장 — context userID > authEnabled 검사 > cli-anonymous 폴백
- Python: `pipelines/auth/dependencies.py` + `tests/test_dependencies.py`, FastAPI `app.add_dependency(Depends(verify_token), exclude=["/health", "/docs"])`
- 기존 `pipelines/main.py` 수정: dependency 등록

**Definition of Done**:
- AC-AUTH-003-1~6 통과
- AC-COMPAT-1 (AuthEnabled=false regression) 통과 — SPEC-AX-CTRL-001 AC-CTRL-UBI-002-A/B/C 결과 동일

### S4 — RBAC Foundation (Priority: High)

**목표**: REQ-AUTH-004 (3-role RBAC) GREEN

**REQ 매핑**: REQ-AUTH-004-S1, REQ-AUTH-004-U1

**TDD 순서**:
- RED: `rbac_test.go` — 3×N 매트릭스 (admin/analyst/viewer × methods/paths), union scope 처리, missing scope → 403
- GREEN: `rbac.go` — scope 파싱 (regex `^iroum-ax:(admin|analyst|viewer)$`), 매트릭스 lookup, `AuthorizeRBAC(ctx, "method/path") error`
- gRPC interceptor + REST middleware에 RBAC 체크 hook 추가 (S3 코드 확장)
- AUTH_FORBIDDEN 액션 audit 기록 (`apps/control-plane/internal/audit/audit.go`에 신규 Action 상수 추가: `ActionAuthRejected`, `ActionAuthForbidden`, `ActionAuthLogout`, `ActionAuthRefreshReuseDetected`)
- Python: `pipelines/auth/dependencies.py`에 `require_role("analyst")` factory dependency 추가

**Definition of Done**:
- AC-AUTH-004-1~5 통과
- audit_logs에 `AUTH_FORBIDDEN` 액션 row 생성 확인 (path/method 포함)

### S5 — Refresh + Logout (Priority: Medium)

**목표**: REQ-AUTH-005 (Token refresh rotation + logout blacklist) GREEN

**REQ 매핑**: REQ-AUTH-005-E1 (Logout), REQ-AUTH-005-U1 (Refresh reuse detection)

**TDD 순서**:
- RED: logout endpoint test — `POST /api/v1/auth/logout`, 양 토큰 blacklist 진입 검증
- GREEN: REST handler 추가 + blacklist.go 활용
- RED: refresh token family test — family_id 발급 → 재사용 시 family 전체 invalidation 검증
- GREEN: refresh family tracking 로직 — Redis key `auth:refresh_family:<family_id>`에 발급된 jti 목록 저장, 재사용 감지 시 모두 blacklist

**Definition of Done**:
- AC-AUTH-005-1~3 통과
- `AUTH_LOGOUT` + `AUTH_REFRESH_REUSE_DETECTED` audit row 생성

### S6 — E2E + Deployment (Priority: Medium)

**목표**: docker-compose / Helm chart 통합, E2E 검증

**산출물**:
- `docker-compose.yml`에 Keycloak 24.x 컨테이너 + dev realm.json mount
- `deployments/helm/iroum-ax/templates/keycloak-statefulset.yaml`
- `deployments/helm/iroum-ax/templates/keycloak-realm-configmap.yaml` — admin/analyst/viewer 클라이언트, scope 정의
- `tests/integration/test_auth_e2e.py` — docker-compose 부팅 → Keycloak admin API로 토큰 발급 → REST 호출 → workflow 생성 → audit_logs에 검증된 user_id 기록 확인 → Celery worker가 envelope `headers.user_id` 추출 후 자체 audit row 작성
- Celery envelope `headers.user_id` 필드 추가 (scheduler/celery_envelope.go) — 기존 envelope golden file 업데이트 필요 (golden file regeneration in S6, not S0)

**Definition of Done**:
- AC-AUTH-E2E-1, AC-AUTH-E2E-2 통과
- 백워드 호환성: `AUTH_ENABLED=false`로 부팅 시 SPEC-AX-CTRL-001 E2E 통과 (test_auth_backward_compat.py)

---

## 2. Sprint 의존성 그래프 (DAG)

```
S0 (scaffolding)
 ├─→ S1 (verifier) ─┐
 │                  ├─→ S3 (middleware) ─→ S4 (RBAC) ─→ S5 (refresh/logout) ─→ S6 (E2E)
 └─→ S2 (OIDC) ─────┘
```

병렬 가능 구간:
- S1 ∥ S2 (verifier와 OIDC discovery는 독립, S3 시작 전 모두 GREEN 필요)
- S4의 audit Action 상수 추가는 S3 후반과 병렬 가능

순차 강제 구간:
- S3 → S4 (RBAC가 미들웨어 hook에 plug-in되어야 함)
- S5 → S6 (logout/refresh가 E2E 테스트 시나리오에 포함됨)

---

## 3. 영향받는 파일 — Sprint별 매핑

S0 (scaffolding):
- `apps/control-plane/internal/auth/` 8개 stub
- `pipelines/auth/` 7개 stub
- `pkg/auth/claims.{go,py}`
- `apps/control-plane/internal/config/config.go` 수정
- `pipelines/config/settings.py` 수정
- `go.mod` / `pyproject.toml` 의존성 추가
- `tests/integration/test_auth_backward_compat.py` (skeleton)

S1 (verifier):
- `apps/control-plane/internal/auth/{verifier.go, jwks_cache.go, blacklist.go, errors.go}`
- `pipelines/auth/{verifier.py, blacklist.py, errors.py}`
- 대응 `*_test.go` / `test_*.py`

S2 (OIDC):
- `apps/control-plane/internal/auth/oidc_client.go`
- `pipelines/auth/oidc_client.py`
- 대응 테스트

S3 (middleware):
- `apps/control-plane/internal/server/{grpc_interceptor.go, rest_middleware.go}`
- `apps/control-plane/internal/server/{grpc_server.go, rest_handler.go}` 수정 (hardcoded user_id 제거)
- `apps/control-plane/internal/audit/recorder.go` 수정 (resolveUserID 확장)
- `apps/control-plane/internal/auth/context.go` (WithUser / UserFromContext 헬퍼)
- `apps/control-plane/cmd/server/main.go` 수정 (wiring)
- `pipelines/auth/dependencies.py`
- `pipelines/main.py` 수정 (dependency 등록)

S4 (RBAC):
- `apps/control-plane/internal/auth/rbac.go` + 테스트
- `apps/control-plane/internal/audit/audit.go` 수정 (4개 Action 상수 추가)
- `apps/control-plane/internal/audit/recorder.go` 수정 (RecordAuthRejected, RecordAuthForbidden 추가)
- `apps/control-plane/internal/server/{grpc_interceptor.go, rest_middleware.go}` 확장 (RBAC hook)
- `pipelines/auth/dependencies.py` 확장 (`require_role` factory)

S5 (refresh/logout):
- `apps/control-plane/internal/server/rest_handler.go` 확장 (`POST /api/v1/auth/logout`)
- `apps/control-plane/internal/auth/refresh_family.go` (신규)
- 대응 테스트

S6 (E2E + deployment):
- `docker-compose.yml` 수정 (Keycloak)
- `deployments/helm/iroum-ax/templates/keycloak-*.yaml` 신규
- `deployments/helm/iroum-ax/values{,-dev}.yaml` 수정
- `apps/control-plane/internal/scheduler/celery_envelope.go` 수정 (`headers.user_id` 추가) + golden file 재생성
- `pipelines/auth/context.py` (Celery task signal handler)
- `pipelines/workers/{ingestion,generation,simulation}_worker.py` 수정 (audit_event INSERT 시 task context user_id 사용)
- `tests/integration/test_auth_e2e.py`

---

## 4. Architectural Trade-offs

### Trade-off 1: OIDC Provider (Decision Already Made — Keycloak)

| 후보 | 자체 호스팅 | 한국 사례 | 망분리 정합 | 통합 복잡도 | 본 SPEC 적합도 |
|------|----------|----------|-----------|-----------|-------------|
| **Keycloak 24.x LTS** | ✓ | 다수 | ✓ | 중 | **선정** |
| 전자정부 표준 인증 | (정부 인프라) | 공공 표준 | △ (인증서 발급 절차) | 높음 | 후속 SPEC |
| Authentik | ✓ | 적음 | ✓ | 중 | 거부 (검증 미흡) |
| Dex | ✓ | 적음 | ✓ | 중 | 거부 (외부 IdP federation 중심) |

**근거**: 검증된 자체 호스팅 + 한국 운영 사례 다수 + OIDC + SAML 동시 지원 (SAML은 후속 시 활용) + RBAC 매핑 용이.

### Trade-off 2: Token Blacklist Storage (Redis vs Postgres)

| 옵션 | Latency p99 | 일관성 | 복잡도 | 본 SPEC |
|------|-----------|------|------|-------|
| **Redis (SET + EXPIREAT)** | < 5ms | Eventual | 낮음 | **선정** |
| Postgres revocation table | < 50ms | Strong | 중 | 거부 |
| In-memory (Go) | < 1ms | Per-instance | 낮음 | 거부 (분산 환경 불일치) |

**근거**: 토큰 만료 시각과 정확히 매칭되는 TTL을 Redis EXPIREAT으로 자연스럽게 구현. SPEC-AX-CTRL-001가 이미 Redis 인스턴스를 사용하므로 추가 의존성 없음.

### Trade-off 3: gRPC Authentication Pattern (Interceptor vs Per-Handler)

| 옵션 | Boilerplate | Coverage | gRPC 표준 |
|------|-----------|---------|---------|
| **UnaryServerInterceptor + StreamServerInterceptor** | 1회 등록 | 100% | ✓ 표준 패턴 |
| Per-handler `auth.Verify(ctx)` 호출 | N회 반복 | 위험 (누락 가능) | 안티패턴 |

**근거**: gRPC는 interceptor 패턴이 표준. 누락 위험 제거 + 단일 진입점.

### Trade-off 4: Python Celery Worker user_id Propagation (Envelope Header vs Argument)

| 옵션 | 기존 코드 변경 | 호환성 | 본 SPEC |
|------|----------|------|-------|
| **Envelope `headers.user_id` 헤더 + task signal handler** | 작음 (3 workers 각각 1줄) | 토큰 미전달 시 envelope 헤더 누락 → 폴백 작동 | **선정** |
| Task 함수 시그니처에 user_id 인자 추가 | 큼 (모든 task signature 변경) | 기존 호출자 모두 수정 | 거부 |

**근거**: Celery `task_prerun` signal handler가 message.headers를 읽어 task-local contextvar에 주입. Worker 함수는 자유롭게 contextvar.get()으로 user_id 조회. 기존 task signature 보존.

### Trade-off 5: RBAC Granularity (Scope vs Role vs ABAC)

| 옵션 | 표현력 | 본 SPEC MVP 적합 |
|------|------|-------------|
| **Scope claim (`iroum-ax:admin` 등)** | 충분 | **선정** |
| 별도 roles claim | 동등 | 거부 (scope가 OIDC 표준) |
| ABAC (resource ownership) | 강력 | 후속 SPEC (Exclusion §11) |

**근거**: OIDC scope claim이 사실상 표준. 3-role MVP에 충분. ABAC은 멀티테넌트 + workflow-level permission 도입 시 추가.

---

## 5. Risk Register

### R-AUTH-001 — JWKS Endpoint Unavailability

**Likelihood**: Medium (Keycloak 단일 인스턴스, 재시작 / 네트워크 partition)
**Impact**: High (모든 인증 요청 실패 → 서비스 중단)
**Detection**: AC-AUTH-001-degraded (JWKS unavailable 시 cached keys 사용 검증)
**Mitigation**:
- 1시간 TTL JWKS 캐시 (REQ-AUTH-001-E2)
- 캐시 만료 후 fetch 실패 시 cached keys로 fallback (stale-while-revalidate)
- HTTP 503 + `Retry-After: 30` 헤더 (cache 자체가 없을 때만)
- 운영 단계 후속 SPEC에서 Keycloak HA (3-replica StatefulSet)
**Residual Risk**: Low (PoC 단계에서 수용)

### R-AUTH-002 — Token Leakage in Logs

**Likelihood**: Medium (개발자가 무심코 `logger.Debug("auth header:", ...)` 작성)
**Impact**: High (토큰 만료 전까지 impersonation 가능)
**Detection**: 로그 스캐닝 정규식 `Bearer [A-Za-z0-9-_\.]{20,}` 검출 시 PR 자동 reject (후속 CI 룰)
**Mitigation**:
- `apps/control-plane/internal/auth/context.go`에 토큰을 context에 저장하지 않음 (검증 후 즉시 폐기, claims만 보관)
- zap log 필드에 `Authorization` 헤더 직접 출력 금지 — `request_id`만 출력
- Python `logging` filter에 토큰 redaction 추가
**Residual Risk**: Low

### R-AUTH-003 — Clock Skew Across Services

**Likelihood**: Medium (K8s pod NTP 미동기 가능)
**Impact**: Medium (정상 토큰이 만료 / 미래 iat로 거부)
**Detection**: AC-AUTH-001-clock_skew_within (±30초 허용 검증)
**Mitigation**:
- REQ-AUTH-001-E1: ±30초 clock skew 허용
- 운영 환경 NTP 동기 docs 추가 (`docs/deployment.md`)
**Residual Risk**: Low

### R-AUTH-004 — Refresh Token Family Race Condition

**Likelihood**: Low (정상 시나리오에서는 발생 안 함, 공격 시도 시 발생)
**Impact**: Medium (정상 사용자가 family invalidation 경험 가능)
**Detection**: AC-AUTH-005-refresh_reuse_detection
**Mitigation**:
- Redis Lua script로 atomic check-and-blacklist (Get family + Validate jti + Blacklist family을 atomic)
- 사용자 friendly 에러 메시지: "다른 기기에서 로그인 감지됨, 재로그인 필요"
**Residual Risk**: Low

### R-AUTH-005 — Keycloak Single Point of Failure (PoC)

**Likelihood**: Medium (단일 인스턴스 운영)
**Impact**: High (Keycloak 죽으면 신규 로그인 불가, 기존 토큰은 만료 전까지만 유효)
**Detection**: Keycloak readiness probe
**Mitigation**:
- 본 SPEC: 단일 인스턴스 수용 (PoC)
- 운영 단계 후속 SPEC: Keycloak HA (3-replica + Postgres backing store)
- 기존 토큰은 JWKS 캐시로 검증 가능하므로 Keycloak 다운 시에도 인증 통과 (단, 신규 로그인만 불가)
**Residual Risk**: Medium (수용 위험)

### R-AUTH-006 — Algorithm Confusion Attack

**Likelihood**: Low (라이브러리가 잘 처리하지만 미스컨피그 시)
**Impact**: Critical (HS256 / none alg 받아들이면 forged token 가능)
**Detection**: AC-AUTH-001-algorithm_rejection
**Mitigation**:
- REQ-AUTH-001-U1: allow-list `[RS256, EdDSA, ES256]`만 허용
- JWKS 응답의 `alg` 필드와 token `alg` 헤더 cross-check
**Residual Risk**: Very Low

### R-AUTH-007 — Backward Compatibility Regression

**Likelihood**: Medium (resolveUserID 로직 확장 시 기존 cli-anonymous 폴백 깨질 수 있음)
**Impact**: High (SPEC-AX-001 / SPEC-AX-CTRL-001 AC 회귀)
**Detection**: `tests/integration/test_auth_backward_compat.py` (S0에서 baseline 작성, S3에서 통과 검증)
**Mitigation**:
- `resolveUserID(ctx, providedUserID, authEnabled)`: authEnabled=false 시 무조건 cli-anonymous 반환 (기존 동작 보존)
- S0에서 regression baseline 테스트 미리 작성
- S3 완료 시 AC-CTRL-UBI-002-A/B/C가 unchanged 결과 반환 검증
**Residual Risk**: Low (자동화 보장)

---

## 6. MX Tag Plan

본 SPEC 구현 시 추가/업데이트될 @MX 태그 (코드 주석 언어: 한국어 per `language.yaml` `code_comments: ko`).

### @MX:ANCHOR (fan_in >= 3 예상, REASON 필수)

| 함수 | 파일 | fan_in 예상 | REASON |
|------|------|----------|-------|
| `VerifyToken(ctx, tokenString) (*Claims, error)` | `auth/verifier.go` | 5+ (gRPC interceptor, REST middleware, logout handler, refresh handler, test) | 모든 인증 경로의 단일 진입점 |
| `auth.UserFromContext(ctx) (*User, bool)` | `auth/context.go` | 10+ (gRPC handlers, REST handlers, audit recorder, RBAC check) | context user 추출 단일 진입점 |
| `AuthorizeRBAC(ctx, methodOrPath) error` | `auth/rbac.go` | 4+ (gRPC interceptor, REST middleware) | RBAC 결정 단일 진입점 |
| `resolveUserID(ctx, providedUserID) string` | `audit/recorder.go` | 8+ (8개 RecordXxx 메서드) | 백워드 호환 invariant (SPEC-AX-001 REQ-UBI-003) |
| `verify_token(authorization: str) -> User` (Python) | `pipelines/auth/dependencies.py` | 5+ (모든 protected endpoint) | Python 측 인증 진입점 |

### @MX:WARN (REASON 필수)

| 위치 | 위험 | REASON |
|------|-----|-------|
| `jwks_cache.go` JWKS fetch goroutine | goroutine 없이 sync.RWMutex만 — 동시성 |  TTL refresh가 첫 요청에서 발생, race 가능 → RWMutex Upgrade lock pattern |
| `refresh_family.go` family invalidation | Lua script atomic 조건문 복잡도 ≥ 12 | Redis race condition mitigation, Lua script eval failure 시 fallback 전략 명시 |
| `rest_middleware.go` token redaction | log line에 Authorization 헤더가 들어가지 않는지 lint 검사 필요 | OWASP A09 mitigation |

### @MX:NOTE

| 위치 | 컨텍스트 |
|------|-------|
| `verifier.go` clock skew 30s | OAuth 2.0 BCP 권고치 |
| `rbac.go` scope prefix `iroum-ax:` | Keycloak realm 설정과 일치, 변경 시 양쪽 동기 |
| `blacklist.go` Redis key prefix `auth:blacklist:` | Go와 Python이 동일 키 스페이스 공유 (cross-language consistency) |
| `audit/audit.go` 신규 Action 상수 4개 | REQ-AUTH-UBI-001 / REQ-AUTH-004 / REQ-AUTH-005 audit enumeration 확장 |

### @MX:TODO (RED → GREEN으로 해소)

- S0 stub 파일 전체에 `@MX:TODO: Sprint S1+ 비즈니스 로직 구현` 표시
- S1~S6 진행에 따라 단계적 제거
- 미해소 TODO는 S6 종료 시 0건이어야 함

---

## 7. Sprint Contract (Harness Thorough Mandate)

`design.yaml` `sprint_contract` 활성화 (harness=thorough). 각 sprint 시작 시 evaluator-active가 Sprint Contract 생성:

| Sprint | Priority Dimension | Pass Threshold |
|--------|------------------|--------------|
| S0 | Consistency (regression baseline) | ≥ 0.75 |
| S1 | Security (algorithm validation, signature) | ≥ 0.80 (security-critical) |
| S2 | Functionality (discovery 정확성) | ≥ 0.75 |
| S3 | Functionality + Consistency (backward compat) | ≥ 0.75 |
| S4 | Security (RBAC enforcement) | ≥ 0.80 |
| S5 | Security (token revocation) | ≥ 0.80 |
| S6 | Functionality (E2E) | ≥ 0.75 |

Strict mode 활성 — must-pass criteria 개별 통과 필요.

---

## 8. Definition of Done (Plan Phase)

- [x] 7개 Sprint 분해 (S0~S6) with priority labels
- [x] Sprint 의존성 DAG (병렬 가능 구간 명시)
- [x] Affected files Sprint별 매핑
- [x] 5개 Architectural Trade-off 결정 (provider, blacklist storage, gRPC pattern, Python propagation, RBAC granularity)
- [x] 7개 Risk register (R-AUTH-001~007) with detection + mitigation + residual
- [x] MX Tag plan (5 ANCHOR + 3 WARN + 4 NOTE + S0 stub TODO)
- [x] Sprint Contract per-sprint priority dimension
- [x] No time estimates (per `agent-common-protocol.md`)
- [x] Backward compatibility regression strategy (R-AUTH-007 mitigation)
