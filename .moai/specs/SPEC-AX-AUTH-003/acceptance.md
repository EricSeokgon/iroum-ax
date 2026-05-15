# SPEC-AX-AUTH-003 — 인수 기준 (acceptance.md)

Given-When-Then 상세 시나리오. 모든 시나리오는 RBAC(AUTH-002) 통과를 전제로 한다(ABAC는 RBAC 뒤에서만 동작). 역할은 scope 토큰 `iroum-ax:(admin|analyst|viewer)`에서 `ParseRolesFromScope`로 도출됨을 가정.

---

## REQ-ABAC-001 — 문서 소유권

### AC-001-1
- Given: analyst(UID=u1)가 RBAC 통과, 문서 자원의 도출 소유자=u1
- When: 해당 문서 접근
- Then: ABAC 허용 → 핸들러 도달(200 계열)

### AC-001-2
- Given: analyst(UID=u1)가 RBAC 통과, 문서 도출 소유자=u2
- When: 해당 문서 접근
- Then: HTTP 403, body `error.code == "ABAC_CONDITION_DENIED"`, 핸들러 미도달

### AC-001-3
- Given: admin이 RBAC 통과, 문서 도출 소유자=u2
- When: 해당 문서 접근
- Then: ABAC 허용(REQ-ABAC-004 Admin 우회) → 핸들러 도달

### AC-001-4
- Given: 요청에서 소유자 식별자 도출 불가(경로/쿼리 없음)
- When: 문서 자원 접근
- Then: 소유권 조건 미적용 — RBAC 결정 유지(투과), DB 조회 수행 안 함

---

## REQ-ABAC-002 — 조직 단위

### AC-002-1
- Given: viewer, 사용자 org_unit=finance(scope `iroum-ax-org:finance`), 자원 org=finance
- When: 자원 접근
- Then: ABAC 허용

### AC-002-2
- Given: viewer, 사용자 org_unit=finance, 자원 org=hr
- When: 자원 접근
- Then: HTTP 403 `ABAC_CONDITION_DENIED`

### AC-002-3
- Given: admin, org_unit=finance, 자원 org=hr
- When: 자원 접근
- Then: 허용(Admin 우회)

### AC-002-4
- Given: 사용자 컨텍스트에 org_unit 속성 부재
- When: 자원 접근
- Then: org 조건 미적용(투과), RBAC 결정 유지

---

## REQ-ABAC-003 — 시간 기반 접근 (KST 고정 UTC+9)

### AC-003-1
- Given: viewer, 현재 10:00 KST
- When: 접근
- Then: 허용

### AC-003-2
- Given: viewer, 현재 08:59 KST
- When: 접근
- Then: HTTP 403 `ABAC_CONDITION_DENIED`

### AC-003-3
- Given: analyst, 현재 18:01 KST
- When: 접근
- Then: HTTP 403 `ABAC_CONDITION_DENIED`

### AC-003-4
- Given: admin, 현재 03:00 KST
- When: 접근
- Then: 허용(Admin 우회)

### AC-003-5
- Given: 컨테이너에 tzdata(zoneinfo) 부재
- When: 시간 조건 평가 실행
- Then: `time.FixedZone("KST", 9*3600)` 사용으로 정상 동작; `time.LoadLocation` 호출 없음(코드 정적 검사로 확인)

---

## REQ-ABAC-004 — Admin 우회

### AC-004-1
- Given: admin이 소유권+org+시간 조건 전부 위반 상태
- When: 접근
- Then: 모든 ABAC 조건 우회, 허용

### AC-004-2
- Given: `ParseRolesFromScope` 결과에 `RoleAdmin` 미포함(analyst/viewer만)
- When: 조건 위반
- Then: 거부(우회 적용 안 됨)

---

## REQ-ABAC-005 — 거부 응답 구별

### AC-005-1
- Given: ABAC 조건 거부 발생
- When: 응답 검사
- Then: HTTP 403 AND body `error.code == "ABAC_CONDITION_DENIED"`

### AC-005-2
- Given: RBAC 권한 부족으로 거부(AUTH-002 경로)
- When: 응답 검사
- Then: body `error.code == "PERMISSION_DENIED"` (ABAC 코드와 상이 — 구별 가능)

### AC-005-3
- Given: ABAC 거부 응답
- When: body 스키마 검사
- Then: `{"error":{"code","message","details"}}` AUTH-002 형식 준수

---

## REQ-ABAC-006 — 외부 의존성 0

### AC-006-1
- Given: 빌드 완료
- When: `go list -deps ./internal/auth/...` 실행
- Then: 신규 외부 모듈 0 (stdlib + 기존 internal 패키지만)

### AC-006-2
- Given: `abac.go` 소스
- When: import 블록 정적 분석
- Then: OPA/Casbin/외부 HTTP 클라이언트 import 0

---

## REQ-ABAC-007 — 감사 로깅

### AC-007-1
- Given: recorder 주입됨, ABAC 거부 발생
- When: 감사 로그 조회
- Then: 거부 row 존재 — 평가된 속성 값 + 거부 사유 포함

### AC-007-2
- Given: recorder=nil, ABAC 거부 발생
- When: 평가 실행
- Then: 패닉 없이 403 반환, 감사 기록 skip(AUTH-002 recorder=nil 관례)

### AC-007-3
- Given: 거부 audit row
- When: action 필드 검사
- Then: `ABAC_CONDITION_DENIED`(S0-1 완료 시) OR 폴백 시 `AUTH_FORBIDDEN`

---

## REQ-ABAC-008 — 비침투 통합

### AC-008-1
- Given: 본 SPEC 머지 커밋
- When: `git diff --name-only` 검사
- Then: `internal/auth/rbac.go`, `chain.go`, `authz_middleware.go`, `middleware.go`, `validator.go`, `errors.go` 변경 0

### AC-008-2
- Given: 본 SPEC 머지
- When: `go test ./internal/auth/... ./cmd/server/...` 실행
- Then: AUTH-001/AUTH-002 기존 테스트 전부 GREEN(회귀 0)

### AC-008-3
- Given: 통합 완료
- When: 변경 파일 목록 검사
- Then: `cmd/server/server.go`(REST 와이어링) + 신규 `internal/auth/abac.go`(+`abac_test.go`) + S0-1 시 `internal/audit/audit.go`(+1줄)로 한정

---

## REQ-ABAC-009 — 기본 안전 / 백워드 호환

### AC-009-1
- Given: `s.cfg.AuthEnabled == false`
- When: 임의 요청
- Then: ABAC 완전 no-op(투과), 기존 AUTH backward-compat 동작과 동일

### AC-009-2
- Given: ABAC 정책 미설정(빈 정책 집합)
- When: RBAC 통과한 요청
- Then: ABAC 투과 — RBAC 결정 유지, 503/거부 발생 안 함

### AC-009-3
- Given: ABAC는 권한 부여를 시도하지 않음
- When: RBAC가 이미 거부한 요청
- Then: ABAC가 allow로 뒤집지 않음(narrowing-only 불변식 — RBAC 거부는 그대로 거부)

---

## Definition of Done

- [ ] REQ-ABAC-001 ~ 009 전체 AC GREEN
- [ ] `internal/auth/abac.go` 커버리지 ≥ 85%
- [ ] AUTH-001/002 회귀 0 (AC-008-2)
- [ ] frozen 파일 무변경 (AC-008-1)
- [ ] 외부 의존성 0 (AC-006-1/2)
- [ ] `time.FixedZone` 사용, `LoadLocation` 미사용 (AC-003-5)
- [ ] @MX:ANCHOR/NOTE/WARN 태그 부착(한국어, @MX:REASON 필수)
- [ ] golangci-lint / gofmt / goimports 통과
- [ ] conventional commit + SPEC-AX-AUTH-003 참조

## 품질 게이트 (TRUST 5)

| 항목 | 기준 | 검증 |
|------|------|------|
| Tested | abac.go ≥ 85%, 회귀 0 | `go test -cover`, AUTH-001/002 suite |
| Readable | godoc + 한국어 주석 + 조건별 타입 분리 | 리뷰 |
| Unified | gofmt/goimports/golangci-lint, auth 패키지 관례 준수 | CI |
| Secured | narrowing-only, fail-safe, 외부 의존성 0, 거부 감사 | AC-006/007/009 |
| Trackable | REQ↔AC↔테스트 추적, SPEC 참조 커밋 | 추적표 |
