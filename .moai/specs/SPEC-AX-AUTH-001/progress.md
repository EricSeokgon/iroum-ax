# SPEC-AX-AUTH-001 Progress Tracker

> Format: Sprint → Phase → 날짜 → AC 완료 수 / 전체 → 에러 델타
> Re-planning Gate: AC 완료율 0% + 3연속 → 트리거

---

## Sprint 1 — REQ-AUTH-001 JWT Validator

### RED phase (2026-05-15)

**파일 생성:**
- `apps/control-plane/internal/auth/validator_test.go` — 19개 테스트 함수 (테이블 포함 실제 케이스 25+)
- `.moai/sprints/SPEC-AX-AUTH-001/sprint-REQ-AUTH-001.md` — Sprint Contract
- `.moai/specs/SPEC-AX-AUTH-001/progress.md` — 이 파일

**의존성 추가:**
- `github.com/golang-jwt/jwt/v5 v5.2.2` → go.mod

**AC 완료 수:** 0 / 16 (RED phase — 미구현 상태가 정상)

**테스트 상태:**
- FAIL: 16개 (stub `errors.New("구현 예정: Sprint 1 GREEN")` 반환)
- PASS: 3개 (헬퍼 자기검증 2 + Blacklisted 에러 반환 확인 1)
- 기존 회귀: 0 (audit/scheduler/server/store/workflow 패키지 OK)

**SF-1 전용 테스트:**
- `TestVerify_IssuerMismatch_Rejected` — iss 불일치 → ErrTokenInvalidIssuer
- `TestVerify_IssuerMatch_Accepted` — iss 일치 → 정상 통과

**SF-2 전용 테스트:**
- `TestVerify_AlgKeyTypeMismatch_Rejected` — kty=RSA + token alg=ES256 → ErrAlgorithmKeyMismatch
- `TestVerify_AlgKeyTypeMatch_RSA_Accepted` — kty=RSA + alg=RS256 → cross-check 통과

**Lesson #4 적용:**
- `ErrNotImplemented` sentinel 패턴 회피
- stub은 `errors.New("구현 예정: Sprint 1 GREEN")` 반환
- GREEN 직후 자연스럽게 PASS로 전환되는 구조

**에러 델타:** +16 신규 FAIL (기대값, RED phase 정상)

**다음 단계:** Sprint 1 GREEN — TokenValidator.Verify 구현

---

## Sprint 0 — Foundation Stubs (완료)

- `validator.go` stub 생성
- `errors.go` sentinel 에러 정의
- `middleware.go` stub 생성
- `rbac.go` stub 생성
- `refresh.go` stub 생성
- `pkg/auth/claims.go` 공유 Claims 구조체
