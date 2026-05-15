# Sprint Contract — REQ-AUTH-004 RBAC 3-role Matrix
## Sprint 5 | SPEC-AX-AUTH-001 | Priority: Security

---

## Scope

REQ-AUTH-004-S1: RBAC 3-role 매트릭스 (admin / analyst / viewer)
REQ-AUTH-004-U1: AUTH_FORBIDDEN 403 + audit 기록

---

## Acceptance Checklist

### ParseRolesFromScope
- [ ] AC-AUTH-004-5: `iroum-ax:admin` → `[RoleAdmin]` (단일 역할 파싱)
- [ ] AC-AUTH-004-4: `iroum-ax:analyst iroum-ax:viewer` → `[RoleAnalyst, RoleViewer]` (다중 역할 union)
- [ ] AC-AUTH-004-5: 미인식 scope는 silently drop
- [ ] 빈 scope → 빈 슬라이스
- [ ] OIDC 표준 scope(`openid`, `offline_access`)는 무시

### EffectivePermissions
- [ ] AC-AUTH-004-3: admin → read:* / write:* / delete:* / audit:read 모두 포함
- [ ] AC-AUTH-004-1: analyst → read:workflow / write:workflow / read:recommendation / write:recommendation / read:audit
- [ ] AC-AUTH-004-2: viewer → read:workflow / read:recommendation (쓰기 없음)
- [ ] AC-AUTH-004-4: analyst+viewer union = analyst (superset)

### Authorize
- [ ] AC-AUTH-004-3: admin 사용자 → write:workflow PASS
- [ ] AC-AUTH-004-2 case A: viewer 사용자 → write:workflow → ErrInsufficientPermission
- [ ] AC-AUTH-004-2 case B: viewer 사용자 → read:workflow → PASS
- [ ] 미들웨어 우회 감지: context에 사용자 없음 → 에러
- [ ] AC-AUTH-004-5: unknown scope → ErrInsufficientPermission
- [ ] AC-AUTH-004-4: analyst+viewer union → write:workflow PASS

### LogForbidden (AUTH_FORBIDDEN audit)
- [ ] audit.Event.Action = audit.ActionAuthForbidden
- [ ] details.method + details.path 포함
- [ ] Event.UserID = 전달된 userID (token sub)
- [ ] details.granted_roles 포함
- [ ] FakeStore 통합: AuditLogs에 기록 확인

---

## Priority Dimension

Security (SPEC-AX-AUTH-001 §4 REQ-AUTH-004, PIPA 접근 제어 의무)

---

## Test File

`apps/control-plane/internal/auth/rbac_test.go`

### 테스트 목록 (18개)

**ParseRolesFromScope (5개)**
- TestParseRolesFromScope_SingleRole
- TestParseRolesFromScope_MultipleRoles
- TestParseRolesFromScope_InvalidRoleIgnored
- TestParseRolesFromScope_EmptyString
- TestParseRolesFromScope_OtherScopes

**EffectivePermissions (4개)**
- TestEffectivePermissions_Admin
- TestEffectivePermissions_Analyst
- TestEffectivePermissions_Viewer
- TestEffectivePermissions_Union

**Authorize (6개)**
- TestAuthorize_AdminHasAllPerms
- TestAuthorize_ViewerCannotWrite_ReturnsErrInsufficientPermission
- TestAuthorize_ViewerCanRead_ReturnsNil
- TestAuthorize_NoUserInContext_ReturnsError
- TestAuthorize_UnknownScope_NoPermission
- TestAuthorize_MultipleRoles_UnionPerms

**LogForbidden (4개)**
- TestLogForbidden_RecordsAuditEvent
- TestLogForbidden_IncludesMethodAndPath
- TestLogForbidden_UserIDFromContext
- TestLogForbidden_IncludesGrantedRoles (+ TestLogForbidden_WithFakeStore_AuditIntegration)

---

## Stub 파일 (Sprint 0 → Sprint 5 GREEN에서 교체)

- `apps/control-plane/internal/auth/rbac.go`:
  - `ParseRolesFromScope`: nil 반환 stub
  - `EffectivePermissions`: nil 반환 stub
  - `Authorize`: `"구현 예정: Sprint 5 GREEN"` 반환 stub
  - `LogForbidden`: `"구현 예정: Sprint 5 GREEN"` 반환 stub

- `apps/control-plane/internal/audit/audit.go`:
  - `ActionAuthForbidden Action = "AUTH_FORBIDDEN"` (신규 추가)

---

## Pass Conditions (GREEN 단계에서 충족)

- 18개 테스트 모두 PASS
- Sprint 1-4 회귀 0건 (Go 156+ 테스트 유지)
- Python 192+ 테스트 유지
- go vet + golangci-lint clean
- ErrInsufficientPermission이 HTTP 403 / gRPC PERMISSION_DENIED로 매핑됨 확인
