# Evaluator-Active Cross-Validation Report: SPEC-AX-AUTH-001

**Evaluation Date**: 2026-05-14
**SPEC**: SPEC-AX-AUTH-001 (SSO/JWT 인증 + 기초 RBAC) v0.1.0
**Harness**: thorough
**Mode**: final-pass (post plan-auditor iter 1)
**Prior Audit**: plan-auditor iter 1 — PASS 0.88
**Evaluator Profile**: default (Functionality 40%, Security 25%, Craft 20%, Consistency 15%)

---

## Evaluation Report

SPEC: SPEC-AX-AUTH-001
Overall Verdict: **CONFIRM (PASS)** — with score dispute (0.88 → 0.782) and 3 new security findings

---

### Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 82/100 | PASS | 36 ACs covering 6 REQs — comprehensive. Gap: refresh token happy path undefined (iroum-ax proxy vs Keycloak delegation ambiguous). |
| Security (25%) | 68/100 | PASS (below 0.75 auth-SPEC target) | `iss` claim validation absent from normative EARS REQs (OWASP JWT requirement). JWKS alg cross-check in NFR only — no dedicated AC. Lua atomicity for refresh family not in REQ/AC. |
| Craft (20%) | 78/100 | PASS | EARS 5-type coverage confirmed. D2/D4/D5 prose issues confirmed. D3 refresh_family.go missing from spec.md §2.1. D6 RBAC perf AC absent. Two additional ACs needed for iss validation and alg cross-check. |
| Consistency (15%) | 85/100 | PASS | D1 (spec.md:L79 dispatcher.go vs celery_envelope.go) confirmed. SPEC-AX-001/CTRL-001 cross-links accurate. audit action names consistent with SPEC-AX-CTRL-001 v0.1.2 D12 resolution (WORKFLOW_TRANSITIONED_TO_RUNNING). |

**Weighted Score**: (82×0.40) + (68×0.25) + (78×0.20) + (85×0.15) = 32.8 + 17.0 + 15.6 + 12.75 = **78.15 / 100 (0.782)**

**Variance from plan-auditor**: -0.098 (0.88 → 0.782)

---

### Agreement Analysis

**D1~D6 plan-auditor findings — 재평가 결과**

| ID | 평가 | Security 관점 추가 |
|----|------|-----------------|
| D1 Major — spec.md:L79 dispatcher.go vs celery_envelope.go | **CONFIRM** (Major). spec-compact.md:L33 및 plan.md:L205 모두 celery_envelope.go를 정확히 지시. spec.md §2.1만 오류. Cross-SPEC artifact celery_envelope_v2.json 재생성이 envelope builder에 귀속됨. | Security 관점 추가 없음. 파일 타겟팅 오류. |
| D2 Minor — REQ-AUTH-UBI-001 trailing prose | **CONFIRM** (Minor). EARS 비준수 산문절. 의미는 명확하나 구조 불일치. | Security 함의 없음. |
| D3 Minor — refresh_family.go spec.md §2.1 누락 | **CONFIRM** (Minor). plan.md S5:L198 + spec-compact.md:L29에는 있으나 spec.md §2.1 Go 테이블에 미기재. | Security 관점 추가: refresh_family.go는 OAuth 2.0 BCP의 핵심 보안 컴포넌트. 영향받는 파일 목록에서 누락되면 Run phase 구현자가 scope를 과소평가할 위험. |
| D4 Minor — REQ-AUTH-005-E1 embedded conditional | **CONFIRM** (Minor). "If either token is malformed, return HTTP 400 (do not partial-blacklist)" → REQ-AUTH-005-U2로 분리 필요. | Security 관점 추가: partial-blacklist 금지는 atomicity invariant로 보안 요구사항에 해당. Minor에서 Security-Minor로 격상 권장. |
| D5 Minor — REQ-AUTH-003-E1 API name 누출 | **CONFIRM** (Minor). `auth.WithUser(ctx, user)` → 구현 세부사항. | Security 함의 없음. |
| D6 Minor — RBAC scope-parse p99 < 1ms AC 부재 | **CONFIRM** (Minor). §7 성능 목표표에 기재만 됨, 전용 AC 없음. | Security 함의 없음. |

---

### Security-Specific Findings (신규)

#### SF-1 (High) — `iss` Claim Per-Token Validation 미규정
**Location**: spec.md §3.2 REQ-AUTH-001-E1 (L154)
**Issue**: REQ-AUTH-001-E1은 exp/nbf/iat/aud 클레임 검증을 명시하나, JWT 표준(RFC 7519 §4.1.1) 및 OWASP JWT Cheat Sheet가 요구하는 `iss` (issuer) 클레임 per-token 검증이 normative EARS REQ에 없다. AC-AUTH-002-3는 startup-time discovery 응답의 issuer URL mismatch를 검사하지만, 이는 각 토큰의 `iss` 클레임을 검증하는 것과 별개다.
**Risk**: coreos/go-oidc의 `provider.Verifier()` 또는 golang-jwt/jwt의 `WithIssuers()` 옵션을 사용하면 자동 검증되지만, SPEC이 이를 명시하지 않으면 구현자가 누락할 수 있다. `iss` 검증 누락 시 cross-realm token 재사용 공격 가능.
**Required Fix**: REQ-AUTH-001-E1에 "validate `iss` claim equals configured OIDC issuer URL" 추가 + AC-AUTH-001-iss 추가 (given: token from different Keycloak realm, when: VerifyToken, then: ErrTokenInvalid + audit reason=issuer_mismatch).

#### SF-2 (High) — JWKS `alg` Cross-Check 전용 AC 부재
**Location**: spec.md §4 NFR table (L232), acceptance.md §8 Edge Case Catalog (L520)
**Issue**: §4 NFR 테이블에 "Algorithm Confusion Attack: alg 헤더 검증을 JWKS 응답의 alg 필드와 매칭 (allow-list)"이 명시되어 있으나 이를 검증하는 전용 AC가 없다. Edge Case Catalog (L520)도 "AC-AUTH-001-5 (HS256 rejection)"으로만 매핑하고 있는데, AC-AUTH-001-5는 token `alg` 헤더가 allow-list 외부인 경우만 다룬다. JWKS 응답의 `alg` 필드와 token `alg`가 *불일치*하는 경우(예: JWKS에 RS256 key가 있는데 token에 ES256을 주장)는 별도 AC가 없다.
**Risk**: Algorithm Confusion Attack 중 JWKS-key-type mismatch 변형(RSA 키를 EC 알고리즘으로 재사용 시도)에 취약할 수 있다. 라이브러리가 처리하더라도 SPEC 수준에서 검증 가능한 AC가 없으면 regression 방어 불가.
**Required Fix**: AC-AUTH-001-alg-cross-check 추가: given: JWKS에 RS256 key(kid=key-rsa), token에 alg=ES256 + same kid; when: VerifyToken; then: reject with algorithm_not_allowed (kid의 알고리즘과 불일치).

#### SF-3 (Medium) — Refresh Token Happy Path 미정의
**Location**: spec.md §3.6 REQ-AUTH-005 (L210~L218), acceptance.md §5
**Issue**: REQ-AUTH-005는 (E1) Logout과 (U1) Refresh Reuse Detection만 정의한다. 그러나 AC-AUTH-005-3(L447)는 `POST /api/v1/auth/refresh` endpoint를 참조한다. iroum-ax가 Keycloak 토큰 갱신 프록시 역할을 하는지(즉, iroum-ax가 /auth/refresh를 직접 구현해서 Keycloak에 relay하는지), 아니면 클라이언트가 Keycloak token endpoint를 직접 호출하는지 SPEC에 명시되지 않았다.
- §7 Out of Scope: "토큰 발급 자체: 본 SPEC은 검증만 담당. 토큰 발급은 Keycloak Authorization Code Flow (PKCE). Console UI에서 처리 — SPEC-AX-CONSOLE-001 범위."라고 명시. 이는 토큰 발급이 Keycloak 직접 호출임을 시사.
- 그러나 refresh family tracking은 iroum-ax가 `auth:refresh_family:<family_id>` Redis key를 관리함을 전제. Keycloak이 직접 refresh를 처리하면 iroum-ax는 family_id를 어떻게 추적하는가?
**Risk**: 구현 모호성 → Run phase에서 잘못된 아키텍처 결정 가능. OAuth 2.0 BCP family invalidation 패턴이 제대로 구현되지 않을 수 있음.
**Required Fix**: §7에 "Token Refresh Proxy" 여부를 명시하거나, Exclusion에 추가. 만약 iroum-ax가 refresh endpoint를 구현한다면 REQ-AUTH-005-E2로 happy path 추가 필요.

#### SF-4 (Low) — Refresh Family Lua Script Atomicity REQ/AC 부재
**Location**: research.md §6 R-AUTH-004 (L277~L283)
**Issue**: R-AUTH-004 mitigation에 "Redis Lua script로 atomic check-and-blacklist" 명시. 그러나 이 atomicity 요구사항은 normative REQ(REQ-AUTH-005-U1) 본문에 없고 AC-AUTH-005-3도 sequential 시나리오만 검증(T+0, T+1, T+2 순차). 동시 요청(race condition) 시나리오 AC 없음.
**Risk**: Lua script 미구현 시 TOCTOU race condition으로 family invalidation이 partial하게 적용될 수 있음. AC 부재 → regression 발생해도 감지 불가.
**Recommended Fix**: REQ-AUTH-005-U1에 "SHALL use atomic Redis operation (Lua script or MULTI/EXEC) to check-and-blacklist" 추가. AC-AUTH-005-race 추가: concurrent goroutines present same refresh token simultaneously → exactly one succeeds, family is fully invalidated (goleak.VerifyNone passable).

---

### Cross-SPEC Consistency Verification

| 항목 | 상태 | 근거 |
|------|------|------|
| SPEC-AX-001 REQ-UBI-003 (audit_logs.user_id + cli-anonymous) | **VERIFIED** | spec.md:L24, REQ-AUTH-UBI-001 WHILE clause, AC-AUTH-UBI-001-C byte-identical guarantee. |
| SPEC-AX-CTRL-001 REQ-CTRL-UBI-002-A (WORKFLOW_CREATED audit) | **VERIFIED** | AC-AUTH-UBI-001-C:L63 + AC-AUTH-E2E-2 anchors SPEC-AX-CTRL-001 AC-CTRL-UBI-002-A/B/C as baseline. |
| SPEC-AX-CTRL-001 REQ-CTRL-UBI-002-B (WORKFLOW_TRANSITIONED_TO_RUNNING) | **VERIFIED** | AC-AUTH-E2E-1:L481 uses "WORKFLOW_TRANSITIONED_TO_RUNNING" — consistent with SPEC-AX-CTRL-001 v0.1.2 D12 resolution. |
| SPEC-AX-CTRL-001 REQ-CTRL-UBI-002-C (cli-anonymous default) | **VERIFIED** | AC-AUTH-UBI-001-C explicitly maps backward compat. test_auth_backward_compat.py confirmed in plan.md. |
| SPEC-AX-CTRL-001 REQ-CTRL-005 Celery envelope user_id propagation | **PARTIAL** | plan.md:L205 + spec-compact.md:L33 correctly target celery_envelope.go. BUT spec.md:L79 still targets dispatcher.go (D1 — unresolved). Golden file regen correctly scheduled in S6. |
| SPEC-AX-CTRL-001 §5 Exclusion #2 해소 | **VERIFIED** | spec.md:L29 명시. |
| SPEC-AX-001 §5 Exclusion #12 해소 | **VERIFIED** | spec.md:L28 명시. |
| audit action names cross-SPEC consistency | **VERIFIED** | AUTH-specific actions (AUTH_REJECTED/FORBIDDEN/LOGOUT/REFRESH_REUSE_DETECTED)가 SPEC-AX-CTRL-001 action enum과 충돌하지 않음. plan.md S4 신규 Action 상수 목록 확인. |

---

### Security Anti-Pattern Check

| Anti-Pattern | Present? | Notes |
|-------------|---------|-------|
| HS256 symmetric key allowed | No | REQ-AUTH-001-U1 명시 거부. AC-AUTH-001-5/6 검증. |
| Algorithm `none` allowed | No | AC-AUTH-001-6. |
| Token stored in logs | No | R-AUTH-002 mitigations + DoD §9 마지막 항목. |
| Missing expiry validation | No | REQ-AUTH-001-E1 명시. |
| Missing audience validation | No | REQ-AUTH-001-E1 명시 (`iroum-ax-control-plane` / `iroum-ax-pipelines`). |
| Missing issuer validation | **YES** | SF-1: normative EARS REQ에 `iss` 클레임 검증 없음. |
| External OAuth allowed (망분리 위반) | No | §5 Exclusion #1 명시 거부. |
| Blacklist bypass possible | No | REQ-AUTH-001-S1 + Redis TTL = token.exp. Redis clock skew edge case (SF-4 관련, Low). |
| Partial logout (atomicity gap) | No | AC-AUTH-005-4 covers malformed token case. |

**Anti-pattern cap 적용 여부**: SF-1 (`iss` validation missing)은 구현 시 라이브러리가 자동 처리할 가능성이 높으나, normative REQ 부재가 확인됨. Anti-pattern으로 분류하기에는 SPEC 수준의 명시 누락이므로 Security dimension 점수 반영(cap 적용 안 함 — 구현에서 반드시 발생한다고 단언 불가).

---

### Evaluation Rubric Anchoring

**Functionality 82/100** (rubric band 75~90):
- 0.75: 대부분 REQ 커버, 일부 gap 존재
- 0.90: 완전한 REQ 커버 + edge case 충분
- 82: 6개 REQ 모두 커버, 36개 AC 적절. SF-3 (refresh happy path 모호) 및 sub-claim 처리 경계에서 5점 차감.

**Security 68/100** (rubric band 50~75):
- 0.50: 주요 보안 항목 누락 다수
- 0.75: OWASP 준수, 대부분 covered, 소수 gap
- 68: 알고리즘 혼동 공격 방어 포함, 블랙리스트 설계 양호, R-AUTH-001~007 risk register 우수. SF-1 (`iss` normative 부재) + SF-2 (alg cross-check AC 부재)로 10점 차감.

**Craft 78/100** (rubric band 75~90):
- D2/D4/D5/D3/D6 + 2개 신규 AC 부재. EARS 구조는 전반적으로 양호. 5점 추가 차감.

**Consistency 85/100** (rubric band 75~100):
- D1 미수정(spec.md L79), 나머지 cross-SPEC 정합 우수. 15점 차감 후 85.

---

### Recommendations

**Must Fix (Run phase 시작 전)**:

1. **D1 (Major)**: spec.md:L79 — `dispatcher.go` → `celery_envelope.go` 정정. S6 시작 전 필수.

2. **SF-1 (High — Security)**: REQ-AUTH-001-E1에 `iss` 클레임 검증 추가:
   - 추가 텍스트: "validate `iss` claim equals the issuer URL from OIDC discovery document"
   - AC-AUTH-001-iss 추가: given: token issued by different Keycloak realm (iss mismatch), when: VerifyToken, then: ErrTokenInvalid + audit reason=issuer_mismatch.
   - S1 RED phase 시작 전 필수.

3. **SF-2 (High — Security)**: AC-AUTH-001-alg-cross-check 추가:
   - given: JWKS kid="key-rsa" has alg=RS256, token claims alg=ES256 with same kid, when: VerifyToken, then: ErrTokenInvalid + audit reason=algorithm_not_allowed.
   - acceptance.md §8 Edge Case Catalog에 항목 추가.

4. **SF-3 (Medium)**: §7 Out of Scope 또는 §5 Exclusions에 refresh token proxy 여부 명시:
   - "iroum-ax는 refresh endpoint를 구현 안 함 — 클라이언트가 Keycloak token endpoint 직접 호출" 또는
   - REQ-AUTH-005-E2로 refresh proxy happy path 명시.
   - Refresh family tracking 아키텍처 근거 명시.

**Recommended (0.1.1 amendment fold-in)**:

5. D2: REQ-AUTH-UBI-001 trailing prose → Unwanted 절 분리 또는 제거.
6. D3: refresh_family.go spec.md §2.1 테이블에 추가.
7. D4: REQ-AUTH-005-E1 embedded conditional → REQ-AUTH-005-U2.
8. D5: spec.md:L182 `auth.WithUser(ctx, user)` 제거 → 구현 중립적 표현.
9. D6: AC-AUTH-004-Performance 추가 (RBAC scope-parse p99 < 1ms).
10. SF-4: REQ-AUTH-005-U1에 Lua script atomicity 요구사항 명시 + race condition AC.

---

### Strengths Worth Preserving

- **Algorithm Confusion Attack 방어 구조**: allow-list (RS256/EdDSA/ES256) + JWKS alg cross-check (§4 NFR) + AC-AUTH-001-5/6 이중 방어. OWASP JWT Cheat Sheet 준수.
- **7개 Risk Register + 인지 편향 체크**: research.md의 Cognitive Bias Check 4종은 드문 우수 사례.
- **Backward compat 삼중 보호**: AC-AUTH-UBI-001-C + AC-AUTH-E2E-2 + R-AUTH-007 + S0 baseline.
- **15개 Exclusion (목표 7개 대비 8개 초과)**: 각각 구체적이고 후속 SPEC ID 명시.
- **Celery envelope propagation 설계**: task_prerun signal handler 패턴이 backward compatible하고 기존 task signature 보존.
- **PISA/PIPA/망분리/감사원 추적성** 컨텍스트가 REQ와 명확히 매핑됨 (spec.md §1.2 운영 컨텍스트 표).

---

### Summary

plan-auditor의 PASS 판정을 **CONFIRM**하되, 보안 차원에서 3개의 신규 발견(SF-1~SF-3)이 이 평가에서 추가로 확인되었다. 이 중 SF-1 (`iss` claim validation 누락)과 SF-2 (JWKS alg cross-check AC 부재)는 인증 SPEC에서 높은 중요도로 간주되며, S1 RED phase 시작 전 normative REQ 및 AC 추가가 필요하다. SF-3 (refresh happy path 모호성)은 아키텍처 결정을 명시함으로써 해소 가능하다.

plan-auditor가 평가한 Clarity/Completeness/Testability/Traceability 차원의 결과는 정확하며, D1~D6 발견 사항도 유효하다. 그러나 security-specific 심층 분석이 추가되어 최종 가중 점수가 0.88 → 0.782로 하향 조정된다. 보안 차원이 0.68로 인증 SPEC 기대치(0.75)에 미치지 못하므로, SF-1/SF-2 해소 후 재평가를 권장한다.
