# SPEC-AX-AUTH-003 — 리서치 요약 (research.md)

소스 코드 직접 분석 결과 중 SPEC 결정에 영향을 준 핵심 발견. 모든 시그니처는 `apps/control-plane/internal/auth/` 및 `internal/audit/`, `cmd/server/server.go`에서 확인됨(phantom-API 방지 — 검증된 API만 SPEC에 기재).

## F1. 역할/권한 모델 — 작업 지시의 가정과 소스가 불일치

작업 지시는 권한을 `read:metrics`/`read:reports`/`write:reports`/`admin:*`로 가정했으나 **소스에는 존재하지 않는다(phantom)**. 실제(`rbac.go:40-60`):
- 역할: `Role` 타입, `RoleAdmin="admin"`, `RoleAnalyst="analyst"`, `RoleViewer="viewer"` (`rbac.go:17-26`).
- 역할은 JWT scope 토큰 `iroum-ax:(admin|analyst|viewer)`에서 `ParseRolesFromScope`(`rbac.go:68`)로 도출.
- 실제 권한: `read:workflow`, `write:workflow`, `delete:workflow`, `read:recommendation`, `write:recommendation`, `read:audit`, `audit:read`.

→ SPEC의 ABAC 조건/자원 모델을 **실제 권한·자원 경로**(`/api/v1/workflows`, `/api/v1/workflows/{id}`, `/api/v1/recommendations/{id}/feedback`, `/api/v1/documents/upload` — `authz_mapping.go:27-57`)에 맞춰 작성. phantom 권한명 미사용.

## F2. `User` 구조체가 JWT Claims 맵을 보유하지 않음 (최대 설계 제약)

- `User`(`middleware.go:25-34`) = `{ UID, Issuer string; Roles, Scopes []string }`. **Claims 맵 없음.**
- `ValidatedToken`(`validator.go:82-89`)에는 `Claims map[string]any`가 있으나, `validatedTokenToUser`(`middleware.go:56-87`, AUTH-001 소유)가 sub/iss/roles/scopes만 추출하고 **나머지 클레임을 폐기**.
- ABAC 미들웨어는 컨텍스트의 `*User`만 접근 가능 → `org_unit` 같은 임의 클레임을 미들웨어 시점에 읽을 수 없음.

→ 결정 D1: 기본은 `org_unit`을 scope 토큰(`iroum-ax-org:<unit>`)으로 인코딩하여 `user.Scopes`로 전달(AUTH-001/002 **무변경**, 망분리 호환). 대안 D1-B는 `User`에 additive `Attributes` 필드 + `validatedTokenToUser` 1줄 — additive 한정 + 회귀 AC 가드.

## F3. `created_by`/`owned_by`는 JWT 클레임이 아님

소유권은 저장된 문서의 속성이며 미들웨어 시점에는 자원 미로드(핸들러 내부 DB 조회). → 결정 D2: 미들웨어 소유권 강제는 요청에서 소유자 도출 가능 시만; 그 외는 핸들러-호출 평가자 API. per-resource DB 조회는 §6 제외(과약속 방지).

## F4. 체인 삽입점 — `chain.go` 수정 불필요

`BuildRESTChain`(`chain.go:43-76`)은 `authnMW(authzMW(handler))`를 캡슐화하고 호출자가 순서를 못 바꾸게 함(@MX:ANCHOR). `server.go:236-241`은 `BuildRESTChain(s.restHandler.Mux(), ...)`로 호출. → 결정 D3: `s.restHandler.Mux()`를 `ABACMiddleware(...)`로 래핑해 `handler` 위치에 전달하면 실행 순서가 자연히 `authn→authz→abac→handler`. `chain.go` **무변경**, server.go(package main) 1곳만 변경 — OBS-001 와이어링 선례와 정합.

## F5. 에러 코드 모델 (AUTH-002)

- RBAC 403: `{"error":{"code":"PERMISSION_DENIED","message","details":{"required","granted"}}}` + `WWW-Authenticate: Bearer` (`authz_middleware.go:210-232`).
- 503 default-deny: `AUTHZ_MAPPING_MISSING`; 500: `AUTHZ_USER_MISSING`.

→ ABAC 거부는 동일 body 형식 + 신규 코드 `ABAC_CONDITION_DENIED`(403)로 RBAC와 구별. 신규 sentinel `ErrABACConditionDenied`는 신규 `abac.go`에 정의(`errors.go`는 AUTH-001 — 무변경; OBS-001 도메인-로컬 선례 정합).

## F6. audit 패키지 — `ActionABACDenied` 부재

`audit.Action`(`audit.go:13`)은 string 타입, 상수에 ABAC 관련 없음(`ActionAuthForbidden="AUTH_FORBIDDEN"` 등만). → 결정 D5: `ActionABACDenied Action = "ABAC_CONDITION_DENIED"`를 **명시적 Sprint 0 additive 작업**으로 추가(phantom 가정 아님). 미완 시 `ActionAuthForbidden` 폴백(degraded, AC-007-3). `audit.NewRecorder(authEnabled bool) *Recorder`(`recorder.go:46`), `auditRecorder` 인터페이스 = `LogForbiddenEvent(...)`(`authz_middleware.go:19-23`); recorder=nil이면 기록 skip(AUTH-002 관례) — ABAC도 동일 적용.

## F7. 타임존 — 망분리 tzdata 부재 위험

scratch 컨테이너에는 zoneinfo가 없을 수 있어 `time.LoadLocation("Asia/Seoul")` 런타임 실패 가능. KST는 DST 없는 고정 UTC+9. → 결정 D4: `time.FixedZone("KST", 9*3600)` 강제, `LoadLocation` 금지(@MX:WARN + AC-003-5).

## F8. 순환 import 위험 없음

신규 `abac.go`는 동일 `auth` 패키지에 배치(`ParseRolesFromScope`/`RoleAdmin`/`User`/`UserFromContext` 패키지-로컬 접근). import은 stdlib + `internal/audit`. `rbac.go:13`이 이미 `internal/audit`를 단방향 import 중 → audit는 auth를 import하지 않음 → 순환 없음. 별도 `internal/abac` 패키지는 export 표면 확대 + 잠재 순환으로 기각.

## 메모리 적용

- `feedback_spec_phantom_api`: 작업 지시의 권한명/`ActionABACDenied`/일부 가정이 phantom임을 소스 대조로 확인, §2.0 검증된 API 표를 단일 진실 원천으로 작성, 미존재 audit 상수는 Sprint 0 scope-add로 명시(은밀한 가정 금지).
- `project_ax_obs_spec`: frozen-asset 갭은 frozen 파일 수정이 아닌 도메인-로컬(신규 파일/additive 옵션)로 해결하는 OBS-001 패턴을 D1/D3/D5에 적용. observer/DI는 실제 순환이 있을 때만 — 여기선 순환 없어 `abac.go`가 `audit` 직접 import(과적용 회피).
