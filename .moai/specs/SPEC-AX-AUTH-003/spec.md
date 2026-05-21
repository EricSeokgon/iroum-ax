---
id: SPEC-AX-AUTH-003
version: "0.1.0"
status: completed
created_at: "2026-05-15"
labels: ["auth", "abac", "security", "air-gapped"]
updated: "2026-05-18"
author: ircp
priority: high
issue_number: null
---

# SPEC-AX-AUTH-003 — 경량 ABAC: 속성 기반 접근 제어 (Lightweight ABAC)

## HISTORY

- v0.1.0 (2026-05-18): Run 완료. evaluator-active PASS 0.905. 30 AC 검증, 98.5% 커버리지.
- 2026-05-15 v0.1.0 (draft): 최초 작성. SPEC-AX-AUTH-001/002 위에 속성 조건을 얹는 경량 ABAC 계층 정의. 외부 의존성 없음(망분리 호환), AUTH-001/002 코드 무변경(server.go 와이어링만), 검증된 API 표 기반.

---

## 1. 개요 (Overview)

### 1.1 목적

기존 RBAC(SPEC-AX-AUTH-002)는 **역할(role)** 단위 결정만 가능하다. "사용자는 자신이 생성한 문서만 접근", "타 조직 데이터 차단", "업무 시간 외 접근 제한" 같은 **속성(attribute) 조건**을 표현할 수 없다. 본 SPEC은 RBAC 위에 **좁히기(narrowing) 전용** 속성 평가 계층을 추가한다.

### 1.2 계층화 (Layering)

```
authn (AUTH-001) → authz/RBAC (AUTH-002) → ABAC 평가 (본 SPEC) → handler
```

ABAC는 **RBAC가 허용한 요청에 한해서만** 추가 속성 조건을 평가한다. ABAC는 **권한을 부여하지 않는다** — 오직 RBAC가 이미 허용한 것을 추가로 **거부**할 수 있을 뿐이다(narrowing filter). 따라서 ABAC 정책이 매칭되지 않으면 RBAC 결정이 그대로 유지된다(pass-through).

### 1.3 핵심 제약

- **외부 서비스 의존성 0** — OPA/Casbin/외부 디렉터리 없음. 망분리(air-gapped) 환경 호환.
- **AUTH-001/AUTH-002 불변식(invariant) 무변경** — `rbac.go`, `chain.go`, `authz_middleware.go`, `middleware.go`, `validator.go`, `errors.go` 코드를 수정하지 않는다. 통합은 `cmd/server/server.go`(package main 와이어링)에서만 수행한다.
- **기본 안전(fail-safe)** — ABAC 정책 미설정 또는 `authEnabled=false`이면 ABAC는 완전 no-op(투과)하여 AUTH 백워드 호환을 보존한다.

---

## 2. 배경 및 컨텍스트 (Background & Context)

### 2.0 검증된 API 표 (Verified-API — single source of truth)

> 본 표의 모든 시그니처는 소스 코드에서 직접 확인되었다. 구현은 이 표를 단일 진실 원천으로 삼는다. 표에 없는 API를 "그럴듯하다"는 이유로 가정해서는 안 된다(phantom-API 금지).

| 심볼 | 시그니처 / 정의 | 위치 |
|------|----------------|------|
| `auth.Role` | `type Role string` | `internal/auth/rbac.go:17` |
| `auth.RoleAdmin/RoleAnalyst/RoleViewer` | `Role = "admin" / "analyst" / "viewer"` | `rbac.go:21-25` |
| `auth.ParseRolesFromScope` | `func(scope string) []Role` (scope 토큰 `iroum-ax:(admin\|analyst\|viewer)` 추출) | `rbac.go:68` |
| `auth.User` | `struct { UID, Issuer string; Roles, Scopes []string }` (**Claims map 미보유**) | `internal/auth/middleware.go:25-34` |
| `auth.UserFromContext` | `func(ctx) (*User, bool)` | `middleware.go:49` |
| `auth.WithUser` | `func(ctx, *User) context.Context` | `middleware.go:40` |
| `auth.validatedTokenToUser` | `func(*ValidatedToken) *User` (unexported, AUTH-001 소유, **전체 claims 폐기**) | `middleware.go:56` |
| `auth.ValidatedToken` | `struct { Claims map[string]any; Subject, Issuer string; Audience, Scopes []string }` | `internal/auth/validator.go:82-89` |
| `auth.BuildRESTChain` | `func(handler http.Handler, validator *TokenValidator, recorder auditRecorder, authEnabled bool, opts ...ChainOption) http.Handler` (`authn(authz(handler))` 강제) | `internal/auth/chain.go:43` |
| `auth.RESTAuthzMiddleware` | `func(recorder auditRecorder, authEnabled bool) func(http.Handler) http.Handler` | `authz_middleware.go:90` |
| `auth.WithGrantedPermission` / `GrantedPermissionFromContext` | additive context annotation 관례 (선례) | `authz_middleware.go:69,74` |
| `auth.auditRecorder` (interface) | `LogForbiddenEvent(ctx, method, path, required, userID string, grantedRoles []Role) error` | `authz_middleware.go:19-23` |
| `auth.ErrInsufficientPermission` | RBAC 403 sentinel | `internal/auth/errors.go:34` |
| RBAC 403 응답 body | `{"error":{"code":"PERMISSION_DENIED","message","details":{"required","granted"}}}` + `WWW-Authenticate: Bearer` | `authz_middleware.go:210-232` |
| 503 default-deny code | `AUTHZ_MAPPING_MISSING` | `authz_middleware.go:240` |
| 500 user-missing code | `AUTHZ_USER_MISSING` | `authz_middleware.go:253` |
| 자원 경로 (REST) | `/api/v1/workflows`, `/api/v1/workflows/{id}`, `/api/v1/recommendations/{id}/feedback`, `/api/v1/documents/upload` | `authz_mapping.go:27-57` |
| 실제 permission 집합 | `read:workflow`, `write:workflow`, `delete:workflow`, `read:recommendation`, `write:recommendation`, `read:audit`, `audit:read` | `rbac.go:40-60` |
| REST 체인 와이어링 | `outerMux.Handle("/", auth.BuildRESTChain(s.restHandler.Mux(), s.tokenValidator, nil, s.cfg.AuthEnabled))` | `cmd/server/server.go:236-241` |
| `audit.Action` | `type Action string`; 상수에 **`ActionABACDenied` 없음** (Sprint 0 추가 필요) | `internal/audit/audit.go:13-50` |
| `audit.NewRecorder` | `func(authEnabled bool) *Recorder` | `internal/audit/recorder.go:46` |
| `audit.AuditTx` (interface) | 호출자 tx 위에서 audit row insert | `internal/audit/recorder.go:28` |

### 2.1 선행 SPEC

- **SPEC-AX-AUTH-001**: JWT issuer 검증 + 알고리즘 혼동 공격 방어 + OAuth 2.0 BCP(refresh rotation/family invalidation). `User`/`ValidatedToken`/`validatedTokenToUser` 정의 소유.
- **SPEC-AX-AUTH-002**: RBAC REST/gRPC 핸들러. 역할 `admin`/`analyst`/`viewer`, 권한 매트릭스, `authz_mapping.go` (method→permission), default-deny(503 `AUTHZ_MAPPING_MISSING`), 체인 순서 `authn → authz → handler` 강제.

### 2.2 갭 (Gap)

RBAC는 역할 전용이다. 다음을 표현할 수 없다:
1. **문서 소유권**: viewer/analyst가 "자신이 만든 문서만" 접근 — RBAC는 `read:workflow`를 가진 모두에게 동일 허용.
2. **조직 단위 격리**: A조직 사용자가 B조직 자원 접근 차단 — RBAC에 org 개념 없음.
3. **업무 시간 제한**: 비-Admin이 09:00–18:00 KST 외 접근 차단 — RBAC에 시간 개념 없음.

### 2.3 핵심 설계 결정 (검증된 제약에서 도출)

#### 결정 D1 — JWT 클레임 전파 (org_unit)

`User` 구조체는 `Claims` 맵을 보유하지 않는다(`middleware.go:25-34`). 전체 클레임은 `validatedTokenToUser`(AUTH-001 소유)에서만 보이고 즉시 폐기된다. ABAC 미들웨어는 컨텍스트의 `*User`만 접근 가능하다.

- **D1-A (권장)**: `org_unit`을 **scope 토큰으로 인코딩**(예: `iroum-ax-org:finance`)하여 `user.Scopes`로 전달. AUTH-001/002 **코드 무변경**, 망분리 호환, 추가 파싱 함수만 신규 `abac.go`에 작성. OBS-001 선례(frozen-asset 갭은 frozen 파일 수정이 아닌 도메인-로컬 해결)와 정합.
- **D1-B (대안, 구현자 선택 가능)**: `User`에 **추가(additive) 필드** `Attributes map[string]string` 신설 + `validatedTokenToUser`에 1줄 additive 채우기. 시그니처/기존 동작 무변경이나 AUTH-001 파일을 건드림 — D1-A로 불가능할 때만 채택하고 AUTH-001 회귀 테스트 GREEN을 AC로 가드.
- 본 SPEC은 **D1-A를 기본**으로 한다.

#### 결정 D2 — 문서 소유권은 JWT 클레임이 아님

`created_by`/`owned_by`는 JWT 클레임이 아니라 **저장된 문서의 속성**이다. 미들웨어 시점에는 자원이 아직 로드되지 않았다(핸들러 내부에서 DB 조회). 따라서:
- 미들웨어 레벨 소유권 강제는 **요청에서 소유자가 도출 가능한 경우에만** 가능(예: 경로/쿼리에 명시).
- 그 외는 **핸들러가 호출하는 평가자 API**로 제공한다. DB-backed per-resource 소유자 조회는 본 SPEC 범위 밖(§6 Exclusions).

#### 결정 D3 — 체인 삽입점

`BuildRESTChain`(`chain.go:43`)은 `authn(authz(handler))`를 캡슐화하며 호출자가 순서를 못 바꾸게 한다(@MX:ANCHOR). 수정 없이 ABAC를 authz 뒤에 넣으려면 **`server.go`에서 handler를 ABAC로 래핑**한다: `BuildRESTChain(ABACMiddleware(...)(s.restHandler.Mux()), ...)`. 실행 순서는 자연히 `authn → authz → abac → handler`가 된다. `chain.go` 무변경.

#### 결정 D4 — KST는 tzdata 비의존

망분리 `scratch` 컨테이너에는 tzdata가 없을 수 있어 `time.LoadLocation("Asia/Seoul")`은 런타임 실패 위험. KST는 DST 없는 고정 UTC+9이므로 `time.FixedZone("KST", 9*3600)` 사용을 강제한다.

#### 결정 D5 — ABAC 거부 audit 액션

`audit.Action`에 `ActionABACDenied` 상수가 **없다**(`audit.go:13-50`). 이는 phantom이 아니라 **명시적 Sprint 0 선행 작업**으로 추가한다(audit 패키지에 `ActionABACDenied Action = "ABAC_CONDITION_DENIED"` 1줄 additive). 추가 전까지 ABAC 거부는 기존 `ActionAuthForbidden` 재사용으로 폴백 가능(degraded, AC로 명시).

---

## 3. 요구사항 (Requirements — EARS)

### REQ-ABAC-001 (문서 소유권)

**WHEN** 요청이 RBAC를 통과하고 문서 자원(`/api/v1/documents/upload` 또는 `/api/v1/workflows/{id}`)을 대상으로 하며 소유자 식별자가 요청 또는 핸들러 컨텍스트에서 도출 가능할 때, **the system SHALL** 비-Admin 역할에 대해 `user.UID`가 자원 소유자와 일치하는 경우에만 접근을 허용하고 불일치 시 거부한다.

### REQ-ABAC-002 (조직 단위)

**WHERE** `org_unit` 속성이 사용자 컨텍스트에 존재할 때, **the system SHALL** 비-Admin 역할에 대해 요청 대상 자원의 조직 단위와 사용자 조직 단위가 일치하지 않으면 교차-조직 접근을 거부한다.

### REQ-ABAC-003 (시간 기반 접근)

**WHEN** 시간 기반 정책이 설정되어 있을 때, **the system SHALL** `RoleViewer` 또는 `RoleAnalyst` 역할(Admin 제외)에 대해 KST(고정 UTC+9) 기준 09:00 이전 또는 18:00 이후 접근을 거부한다.

### REQ-ABAC-004 (Admin 우회)

**WHERE** 사용자가 `RoleAdmin` 역할을 보유할 때(`ParseRolesFromScope` 결과에 `RoleAdmin` 포함), **the system SHALL** 모든 ABAC 속성 조건을 우회하고 RBAC 결정을 그대로 따른다.

### REQ-ABAC-005 (거부 응답)

**IF** ABAC 평가가 실패하면 **THEN the system SHALL** HTTP 403과 에러 코드 `ABAC_CONDITION_DENIED`를 반환하며, 이는 RBAC 403(`PERMISSION_DENIED`)과 코드 값으로 구별 가능해야 한다. 응답 body는 AUTH-002 형식(`{"error":{"code","message","details"}}`)을 따른다.

### REQ-ABAC-006 (의존성 0)

**The system SHALL** ABAC 평가자를 외부 서비스 의존성 0으로 구현한다(stdlib + 기존 internal 패키지만). 신규 외부 모듈 의존성을 도입하지 않아야 한다.

### REQ-ABAC-007 (감사 로깅)

**WHEN** ABAC 결정(허용/거부)이 내려질 때, **the system SHALL** 거부 결정을 감사 추적에 기록한다(평가된 속성 값과 거부 사유 포함). audit 액션은 D5의 `ActionABACDenied`이며, recorder가 nil이면 기록을 skip한다(AUTH-002 recorder=nil 관례 정합).

### REQ-ABAC-008 (비침투 통합)

**The system SHALL** AUTH-001 및 AUTH-002 구현 코드를 변경하지 않는다. 통합 지점은 `cmd/server/server.go`의 REST 체인 와이어링 1곳(handler 래핑)으로 한정한다.

### REQ-ABAC-009 (기본 안전 / 백워드 호환 — Ubiquitous)

**The system SHALL** `authEnabled=false`이거나 매칭되는 ABAC 정책이 없을 때 ABAC를 완전 no-op(투과)로 동작시켜 AUTH-001/002 백워드 호환과 RBAC 결정을 보존한다(ABAC는 권한을 부여하지 않으며, 정책 부재 시 503/거부를 발생시키지 않는다).

---

## 4. 인수 기준 (Acceptance Criteria)

> 형식: EARS 패턴 (Event-driven: "When ..., the system shall ..."; State-driven: "Where ..., the system shall ..."; Unwanted: "If ..., then the system shall ..."; Ubiquitous: "The system shall ..."). 상세 Given-When-Then 시나리오는 `acceptance.md` 참조.

### REQ-ABAC-001
- **AC-001-1** When an analyst user with UID=u1 has passed RBAC and accesses a document with owner=u1, the system shall permit the request.
- **AC-001-2** When an analyst user with UID=u1 has passed RBAC and accesses a document with owner=u2, the system shall deny the request with HTTP 403 and code `ABAC_CONDITION_DENIED`.
- **AC-001-3** Where the user holds the admin role, when the user has passed RBAC and accesses a document with owner=u2, the system shall permit the request (REQ-ABAC-004 bypass).
- **AC-001-4** If the owner identifier cannot be derived from the request when a document resource is accessed, then the system shall not apply the ABAC ownership condition and shall preserve the RBAC decision (pass-through; DB lookup is out of scope).

### REQ-ABAC-002
- **AC-002-1** When a viewer user with org_unit=finance accesses a finance resource, the system shall permit the request.
- **AC-002-2** When a viewer user with org_unit=finance accesses an hr resource, the system shall deny the request with HTTP 403 and code `ABAC_CONDITION_DENIED`.
- **AC-002-3** Where the user holds the admin role, when an admin user with org_unit=finance accesses an hr resource, the system shall permit the request (REQ-ABAC-004 bypass).
- **AC-002-4** If the org_unit attribute is absent when a resource is accessed, then the system shall not apply the org condition and shall preserve the RBAC decision (pass-through).

### REQ-ABAC-003
- **AC-003-1** When a viewer user accesses at 10:00 KST, the system shall permit the request.
- **AC-003-2** When a viewer user accesses at 08:59 KST, the system shall deny the request with HTTP 403 and code `ABAC_CONDITION_DENIED`.
- **AC-003-3** When an analyst user accesses at 18:01 KST, the system shall deny the request with HTTP 403 and code `ABAC_CONDITION_DENIED`.
- **AC-003-4** Where the user holds the admin role, when an admin user accesses at 03:00 KST, the system shall permit the request (REQ-ABAC-004 bypass).
- **AC-003-5** When the system evaluates time-based access conditions in an environment without tzdata, the system shall correctly determine KST business hours without a runtime error.

### REQ-ABAC-004
- **AC-004-1** Where the user holds the admin role, when an admin user violates all ABAC conditions and accesses a resource, the system shall permit the request (all conditions bypassed).
- **AC-004-2** If the role set does not include `RoleAdmin` when an ABAC condition is violated, then the system shall deny the request (no bypass).

### REQ-ABAC-005
- **AC-005-1** When an ABAC denial occurs, the system shall return HTTP 403 with body code `ABAC_CONDITION_DENIED`.
- **AC-005-2** When an RBAC denial occurs, the system shall return body code `PERMISSION_DENIED`, distinguishable from the ABAC code.
- **AC-005-3** When an ABAC denial response is produced, the system shall preserve the AUTH-002 body format (`error.code/message/details`).

### REQ-ABAC-006
- **AC-006-1** When `go list -deps ./internal/auth/...` is executed against the build, the system shall introduce zero new external modules (stdlib + existing internal only).
- **AC-006-2** When static analysis inspects the `abac.go` imports, the system shall contain zero OPA/Casbin/HTTP-client imports.

### REQ-ABAC-007
- **AC-007-1** Where a recorder is injected, when an ABAC denial occurs, the system shall record a deny row containing the evaluated attribute values and the deny reason.
- **AC-007-2** If the recorder is nil when an ABAC denial occurs, then the system shall return HTTP 403 without panic (recording skipped).
- **AC-007-3** Where Sprint 0 D5 (ActionABACDenied constant) is complete, when an ABAC deny event occurs, the system shall record audit action=`ABAC_CONDITION_DENIED`.
- **AC-007-4** Where Sprint 0 D5 is not yet applied, when an ABAC deny event occurs, the system shall record audit action=`AUTH_FORBIDDEN` as a degraded fallback.

### REQ-ABAC-008
- **AC-008-1** When `git diff` is inspected after this SPEC is merged, the system shall show zero changes to `rbac.go/chain.go/authz_middleware.go/middleware.go/validator.go/errors.go`.
- **AC-008-2** When the existing AUTH-001/002 tests are executed after this SPEC is merged, the system shall keep them all GREEN (zero regression).
- **AC-008-3** When the changed files are inspected after integration, the system shall limit changes to the `cmd/server/server.go` REST wiring plus the new `internal/auth/abac.go` (+test).

### REQ-ABAC-009
- **AC-009-1** Where `authEnabled=false`, when any request is processed, the system shall make ABAC a no-op (pass-through), preserving existing AUTH backward compatibility.
- **AC-009-2** If no ABAC policy is configured when an RBAC-passed request is processed, then the system shall make ABAC pass through (RBAC decision preserved, no 503/denial).
- **AC-009-3** If an RBAC-denied request is processed, then the system shall not let ABAC flip it to allow (narrowing-only).

---

## 5. 기술적 접근 (Technical Approach)

### 5.1 아키텍처

```
authn (AUTH-001) → authz/RBAC (AUTH-002) → ABACMiddleware (신규) → handler
                                                  │
                                                  └─ ABACEvaluator.Evaluate(ctx, user, req) → (allow bool, denyReason string)
```

### 5.2 패키지 배치

신규 파일 `internal/auth/abac.go` (+ `abac_test.go`). **동일 `auth` 패키지**에 둔다 — `ParseRolesFromScope`/`RoleAdmin`/`User`/`UserFromContext`를 패키지-로컬로 접근. 별도 `internal/abac` 패키지는 export 표면 확대 + 순환 import 위험으로 기각. import: stdlib(`context,time,net/http,errors,encoding/json`) + `internal/audit`(rbac.go가 이미 단방향 import — 순환 없음).

### 5.3 핵심 타입 (설계 — 구현 세부는 Run 단계에서 확정)

```go
// ABACCondition — 평가 가능한 단일 속성 조건
type ABACCondition interface {
    Evaluate(ctx context.Context, user *User, req *http.Request) (allow bool, reason string)
}

// OwnershipCondition — user.UID vs 요청 도출 소유자
type OwnershipCondition struct { OwnerFromRequest func(*http.Request) (owner string, ok bool) }

// OrgUnitCondition — user org_unit vs 자원 org_unit
type OrgUnitCondition struct { UserOrg func(*User) string; ResourceOrg func(*http.Request) string }

// TimeWindowCondition — KST 업무 시간 (FixedZone)
type TimeWindowCondition struct { StartHour, EndHour int } // TZ = time.FixedZone("KST", 9*3600)

// ABACPolicy — 특정 자원 패턴에 대한 조건 묶음
type ABACPolicy struct { PathPrefix string; Conditions []ABACCondition }

// ABACEvaluator — 정책 집합 평가; Admin 우회 + narrowing-only 보장
type ABACEvaluator struct { policies []ABACPolicy; recorder auditRecorder }
func (e *ABACEvaluator) Evaluate(ctx context.Context, user *User, req *http.Request) (allow bool, reason string)

// ABACMiddleware — server.go에서 handler 래핑용
func ABACMiddleware(e *ABACEvaluator, authEnabled bool) func(http.Handler) http.Handler
```

평가 규칙: (1) `authEnabled=false` 또는 매칭 정책 없음 → 투과(REQ-ABAC-009). (2) `RoleAdmin` 보유 → 투과(REQ-ABAC-004). (3) 매칭 정책의 모든 조건 AND 평가; 하나라도 deny → 403 `ABAC_CONDITION_DENIED` + audit. (4) ABAC는 절대 allow를 부여하지 않음(RBAC가 이미 게이트).

ABAC 거부 시 `auditRecorder.LogForbiddenEvent`의 `required` 파라미터는 거부된 속성 조건 설명을 전달한다 (예: `ownership:mismatch`, `org_unit:cross-org`, `time_window:outside_hours`).

### 5.4 통합 (server.go, 유일 변경 지점)

`cmd/server/server.go:236-241` 의
`outerMux.Handle("/", auth.BuildRESTChain(s.restHandler.Mux(), s.tokenValidator, nil, s.cfg.AuthEnabled))`
를
`outerMux.Handle("/", auth.BuildRESTChain(auth.ABACMiddleware(abacEvaluator, s.cfg.AuthEnabled)(s.restHandler.Mux()), s.tokenValidator, nil, s.cfg.AuthEnabled))`
로 변경. `chain.go` 무변경, 실행 순서 `authn→authz→abac→handler` 자연 보장.

### 5.5 테스트 요구

- `internal/auth/abac.go` 커버리지 ≥ 85% (TRUST 5 Tested).
- 테이블 주도 테스트, `t.Parallel()`, `httptest`로 미들웨어 통합 검증.
- AUTH-001/002 기존 테스트 회귀 0 (AC-008-2).
- import-graph 단정(`go list -deps`)으로 외부 의존성 0 가드(AC-006-1).

### 5.6 Sprint 0 선행 (검증 기반, phantom 아님)

- **S0-1**: `internal/audit/audit.go`에 `ActionABACDenied Action = "ABAC_CONDITION_DENIED"` 1줄 additive 추가(D5). 미수행 시 `ActionAuthForbidden` 폴백(degraded, AC-007-3).
- **S0-2**: org_unit 전달 방식 확정 — D1-A(scope 인코딩, 기본) vs D1-B(User.Attributes additive). 구현 시작 전 결정 기록.

---

## 6. 제외 범위 (Exclusions — What NOT to Build)

- **OPA/Rego 정책 언어** — Phase 3 이연. 망분리에서 OPA sidecar 운용 부담.
- **Casbin 어댑터** — 망분리 비호환(모델/정책 외부 로딩 가정). 채택 안 함.
- **런타임 동적 정책 리로드** — 정적 설정만. hot-reload는 보안 결정 누락 위험(AUTH-002 code-as-config 철학 정합).
- **외부 디렉터리 서비스 속성 동기화** — LDAP/AD/SCIM 연동 없음(망분리).
- **gRPC 엔드포인트 ABAC** — 본 SPEC은 REST 전용. gRPC ABAC 인터셉터는 후속 SPEC 이연.
- **DB-backed per-resource 소유자 조회** — 미들웨어에서 자원 DB 조회 없음(D2). 요청 도출 불가 소유권은 핸들러-호출 평가자 API로만 제공, per-resource 조회는 후속.
- **`User` 구조체 클레임 맵 노출 / AUTH-001 클레임 파이프라인 재설계** — D1-A 기본; D1-B는 additive 한정.

---

## 7. MX 태그 계획 (MX Tag Plan)

| 심볼 | 태그 | 사유 |
|------|------|------|
| `ABACEvaluator.Evaluate` | `@MX:ANCHOR` | 모든 요청에서 미들웨어가 호출하는 ABAC 결정 단일 진입점(fan_in ≥ 3: 미들웨어/핸들러-호출/테스트). @MX:REASON 필수. |
| `ABACMiddleware` | `@MX:NOTE` | 체인 삽입점 — `authn→authz→abac→handler` 순서가 server.go 래핑으로 보장됨을 명시. |
| `TimeWindowCondition.Evaluate` | `@MX:WARN` | 타임존 처리 — tzdata 비의존 `time.FixedZone("KST",9*3600)` 강제, `LoadLocation` 금지(망분리 런타임 실패 방지). @MX:REASON 필수. |

태그 설명 언어: `code_comments: ko` (language.yaml) → 한국어.

---

## 8. TRUST 5 매핑

- **Tested**: `abac_test.go` 커버리지 ≥ 85%, 테이블 주도, AUTH-001/002 회귀 0(AC-008-2).
- **Readable**: 조건별 타입 분리, godoc, 한국어 주석(language.yaml).
- **Unified**: `gofmt`/`goimports`/`golangci-lint` 통과, 기존 auth 패키지 관례(함수형 옵션/sentinel error/context annotation) 준수.
- **Secured**: narrowing-only(권한 부여 불가), fail-safe(정책 부재 시 RBAC 결정 유지), 외부 의존성 0(OWASP A06 공급망 위험 최소화), 거부 감사 로깅.
- **Trackable**: REQ-ABAC-001~009 ↔ AC ↔ 테스트 추적, conventional commit + SPEC-AX-AUTH-003 참조.
