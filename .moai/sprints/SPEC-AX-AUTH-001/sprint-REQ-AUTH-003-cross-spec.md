# Sprint Contract — REQ-AUTH-003-E3 Python Middleware + REQ-AUTH-UBI-001 Cross-SPEC Envelope

> Phase: Sprint 4 RED
> Date: 2026-05-15
> Sprint type: Cross-SPEC (SPEC-AX-AUTH-001 × SPEC-AX-CTRL-001)

---

## 1. Acceptance Checklist

### 1.1 Python FastAPI verify_token (REQ-AUTH-003-E3)

| 항목 | 기준 | 검증 방법 |
|------|------|-----------|
| valid RS256 Bearer → ValidatedToken 반환 | `isinstance(result, ValidatedToken)` | unit test |
| 만료 토큰 → HTTPException 401 | `status_code == 401` | unit test |
| 서명 검증 실패 → HTTPException 401 | `status_code == 401` | unit test |
| aud 불일치 → HTTPException 401 | `status_code == 401` | unit test |
| iss 불일치 → HTTPException 401 (SF-1) | `status_code == 401` | unit test |
| HS256 알고리즘 → HTTPException 401 | `status_code == 401` | unit test |
| Authorization 헤더 누락 → HTTPException 401 | `status_code == 401` | unit test |
| 잘못된 JWT 형식 → HTTPException 401 | `status_code == 401` | unit test |
| AUTH_ENABLED=false → sub='cli-anonymous' | `result.subject == 'cli-anonymous'` | unit test |
| 401 응답에 WWW-Authenticate Bearer 포함 | `'Bearer' in headers['WWW-Authenticate']` | unit test |
| TokenValidator.verify() → ValidatedToken Pydantic | `isinstance(result, ValidatedToken)` | unit test |
| kty=RSA + alg=ES256 → AlgorithmKeyMismatchError (SF-2) | `AlgorithmKeyMismatchError` 발생 | unit test |

### 1.2 Go Celery Envelope user_id (REQ-AUTH-UBI-001)

| 항목 | 기준 | 검증 방법 |
|------|------|-----------|
| BuildEnvelope(userID='kepco-analyst-001') → headers.user_id 설정 | `headers['user_id'] == 'kepco-analyst-001'` | unit test |
| BuildEnvelope(userID='') → headers.user_id='cli-anonymous' | `headers['user_id'] == 'cli-anonymous'` | unit test |
| authuser 골든 파일 일치 | JSON 구조 동등성 | golden file test |
| anon 골든 파일 일치 | JSON 구조 동등성 | golden file test |
| 기존 celery_envelope_v2.json 회귀 무손상 | 기존 필드 동일 | regression guard |

---

## 2. Priority Dimension

**이번 Sprint 우선 차원: Security + Backward Compatibility**

- SF-1 (iss 검증) + SF-2 (alg/kty cross-check): Algorithm Confusion Attack 방어 핵심
- REQ-AUTH-UBI-001 backward compat: AUTH_ENABLED=false → cli-anonymous 폴백 유지
- Sprint 6 CTRL 회귀 가드: 기존 15개 테스트 불변

---

## 3. Test Seams

### Python
- `pipelines/auth/validator.py` → `TokenValidator.verify()` stub (NotImplementedError)
- `pipelines/auth/dependencies.py` → `verify_token()` stub (HTTPException 501 / AUTH_ENABLED=false 분기)

### Go
- `apps/control-plane/internal/scheduler/dispatcher.go` → `BuildEnvelope()` stub (4-param)
- 신규 파라미터: `userID string` (5번째) — GREEN에서 추가

---

## 4. Pass Conditions

| 언어 | RED 테스트 수 | GREEN 후 기대 |
|------|-------------|--------------|
| Python | 11개 (FAIL) + 2개 (PASS — missing header + auth disabled) | 13개 전체 PASS |
| Go | 5개 (FAIL — 컴파일 에러) | 5개 전체 PASS + 기존 15개 회귀 유지 |

---

## 5. Cross-SPEC 아티팩트

| 파일 | 상태 | 비고 |
|------|------|------|
| `testdata/celery_envelope_v2.json` | 유지 (무손상) | Sprint 6 CTRL 회귀 가드 |
| `testdata/celery_envelope_v2_anon.json` | 신규 생성 | user_id='cli-anonymous' |
| `testdata/celery_envelope_v2_authuser.json` | 신규 생성 | user_id='kepco-analyst-001' |

---

## 6. Scope Boundaries (DO NOT MODIFY in GREEN)

- `pipelines/auth/oidc_client.py` — Sprint 5에서 처리
- `pipelines/auth/blacklist.py` — Sprint 5에서 처리
- `pipelines/config/settings.py` — settings.auth_enabled 읽기만 허용 (기존 코드 활용)
- `apps/control-plane/internal/auth/` — Sprint 1-3에서 이미 처리
- Python Celery worker 실제 user_id 소비 코드 — 향후 별도 SPEC

---

Version: 1.0.0
SPEC: SPEC-AX-AUTH-001
REQ: REQ-AUTH-003-E3, REQ-AUTH-UBI-001
