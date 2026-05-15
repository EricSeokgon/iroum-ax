# SPEC-AX-AUTH-003 — 구현 계획 (plan.md)

## 1. 구현 개요

기존 RBAC(AUTH-002) 위에 **좁히기 전용(narrowing-only)** ABAC 계층을 신규 파일 `internal/auth/abac.go`로 추가한다. AUTH-001/002 코드는 수정하지 않으며, 통합은 `cmd/server/server.go` REST 체인 와이어링 1곳(handler 래핑)으로 한정한다.

설계 단일 진실 원천: `spec.md §2.0 검증된 API 표`. 표에 없는 API는 가정 금지.

## 2. 마일스톤 (우선순위 기반 — 시간 추정 없음)

### Sprint 0 — 선행 검증 작업 (Priority: High, 차단)

| ID | 작업 | 산출물 / 영향 파일 | 검증 |
|----|------|--------------------|------|
| S0-1 | `audit.Action`에 `ActionABACDenied Action = "ABAC_CONDITION_DENIED"` additive 추가 | `internal/audit/audit.go` (+1줄, additive) | `go build`; audit 기존 테스트 GREEN |
| S0-2 | org_unit 전달 방식 확정 (D1-A scope 인코딩 = 기본 / D1-B User.Attributes = 대안) | 결정 기록 (plan.md 갱신) | 결정 명문화 |

> S0-1 미완료 시 `ActionAuthForbidden` 폴백 가능(degraded). S0-2는 구현 시작 전 필수 확정.

### Sprint 1 — ABAC 코어 (Priority: High)

| ID | 작업 | 영향 파일 |
|----|------|-----------|
| S1-1 | `ABACCondition` 인터페이스 + `ABACEvaluator`(Admin 우회 + narrowing-only + 정책 매칭) | `internal/auth/abac.go` (신규) |
| S1-2 | `TimeWindowCondition` (`time.FixedZone("KST",9*3600)`, LoadLocation 금지) | `internal/auth/abac.go` |
| S1-3 | `OrgUnitCondition` (D1-A: scope 토큰 `iroum-ax-org:*` 파서) | `internal/auth/abac.go` |
| S1-4 | `OwnershipCondition` (요청 도출 소유자 vs `user.UID`) | `internal/auth/abac.go` |
| S1-5 | sentinel `ErrABACConditionDenied` + 403 응답 writer (AUTH-002 body 형식 재현) | `internal/auth/abac.go` |

### Sprint 2 — 미들웨어 & 통합 (Priority: High)

| ID | 작업 | 영향 파일 |
|----|------|-----------|
| S2-1 | `ABACMiddleware` (authEnabled=false/정책부재 투과, 거부 시 403+audit) | `internal/auth/abac.go` |
| S2-2 | `server.go` REST 와이어링 — `s.restHandler.Mux()`를 `ABACMiddleware`로 래핑 | `cmd/server/server.go` (와이어링만) |
| S2-3 | ABAC 정책 부트스트랩(정적 구성; 동적 리로드 없음) | `cmd/server/server.go` 또는 config |

### Sprint 3 — 테스트 & 검증 (Priority: High)

| ID | 작업 | 영향 파일 |
|----|------|-----------|
| S3-1 | `abac_test.go` 테이블 주도, 커버리지 ≥ 85% | `internal/auth/abac_test.go` (신규) |
| S3-2 | `httptest` 미들웨어 통합 테스트(소유권/org/시간/Admin우회/투과) | `abac_test.go` |
| S3-3 | AUTH-001/002 기존 테스트 회귀 0 확인 | `go test ./internal/auth/... ./cmd/server/...` |
| S3-4 | import-graph 단정(`go list -deps`)으로 외부 의존성 0 가드 | `abac_test.go` 또는 CI |
| S3-5 | `git diff`로 frozen 파일 무변경 확인(AC-008-1) | CI 가드 |

### Sprint 4 — MX 태그 & 마감 (Priority: Medium)

| ID | 작업 |
|----|------|
| S4-1 | `@MX:ANCHOR` `ABACEvaluator.Evaluate`, `@MX:NOTE` `ABACMiddleware`, `@MX:WARN` `TimeWindowCondition.Evaluate` (한국어, @MX:REASON 필수) |
| S4-2 | golangci-lint / gofmt / goimports 통과 |
| S4-3 | conventional commit (SPEC-AX-AUTH-003 참조) |

## 3. 기술적 접근 요약

- **체인 삽입**: `chain.go` 무변경. server.go에서 `BuildRESTChain(ABACMiddleware(...)(mux), ...)`. 실행 순서 자연 보장.
- **타임존**: `time.FixedZone("KST", 9*3600)` — tzdata 비의존(망분리 scratch 호환).
- **org_unit (기본 D1-A)**: scope 토큰 `iroum-ax-org:<unit>` → `user.Scopes`에서 파싱, AUTH-001/002 무변경.
- **소유권**: 미들웨어는 요청에서 도출 가능한 소유자만 평가. DB 조회 범위 밖(§6).
- **fail-safe**: 정책 부재/authEnabled=false → 투과. ABAC는 allow 부여 불가(narrowing-only).
- **audit**: `ActionABACDenied`(S0-1) 또는 폴백 `ActionAuthForbidden`; recorder=nil이면 skip(AUTH-002 관례).

## 4. 리스크 및 완화

| 리스크 | 영향 | 완화 |
|--------|------|------|
| `User`에 Claims 미보유로 org_unit 접근 불가 | High | D1-A scope 인코딩 기본(AUTH-001 무변경). D1-B는 additive 한정 + 회귀 AC 가드 |
| 미들웨어에서 자원 소유자 미확보 | Medium | D2: 요청 도출 가능 시만 미들웨어 강제, 그 외 핸들러-호출 평가자; DB 조회 §6 제외 |
| scratch 컨테이너 tzdata 부재 → LoadLocation 실패 | High | `time.FixedZone` 강제(AC-003-5), `LoadLocation` 금지 lint/리뷰 가드 |
| `audit.ActionABACDenied` 부재 | Medium | S0-1 additive 추가; 미완 시 `ActionAuthForbidden` 폴백(AC-007-3) |
| frozen 파일 우발 수정 | High | AC-008-1 `git diff` CI 가드 + AC-008-2 회귀 테스트 |
| ABAC가 RBAC 거부를 뒤집을 위험 | High | narrowing-only 불변식(REQ-ABAC-009/AC-009-3) + 테스트로 강제 |

## 5. 검증된 API 의존 (재확인 — phantom 금지)

`spec.md §2.0` 표가 단일 진실 원천. 구현 중 표에 없는 auth/audit 심볼이 필요해지면, "그럴듯한 이름"으로 가정하지 말고 소스 grep/Read로 확인 후 표를 갱신하거나 Sprint 0 선행 작업(scope-add)으로 명시한다.
