# Evaluation Report
SPEC: SPEC-AX-AUTH-002 v0.1.1
Date: 2026-05-15
Harness: thorough
Overall Verdict: **FAIL**

---

## Dimension Scores

| Dimension | Score | Verdict | Evidence |
|-----------|-------|---------|----------|
| Functionality (40%) | 75/100 | FAIL | /metrics handler absent from Mux() + plan inconsistency on AC-AUTH2-Metrics-Admin case A (admin 200 cannot be achieved with current plan) |
| Security (25%) | 70/100 | FAIL | **HARD THRESHOLD VIOLATION** (auth SPEC standard ≥ 75): `read:metrics` permission not defined in AUTH-001 rbac.go permissionMatrix — admin Authorize(ctx, "read:metrics") returns ErrInsufficientPermission; /metrics inaccessible in production |
| Craft (20%) | 80/100 | PASS | 25 ACs binary-testable; DoD 17-item checklist comprehensive; MX tag plan specific; benchmark assertions present; -race + goleak planned |
| Consistency (15%) | 77/100 | PASS | EARS format consistent; D8 cosmetic confirmed minor; AC-AUTH-E2E-3 cross-SPEC reference inconsistency (AUTH-001 acceptance.md §6에 AC-AUTH-E2E-3 항목 미존재) |

**Weighted Score**: 0.40×75 + 0.25×70 + 0.20×80 + 0.15×77 = 30.0 + 17.5 + 16.0 + 11.55 = **75.05/100**

**Variance from plan-auditor 0.92**: -17 points. The plan-auditor evaluated EARS format, REQ numbering, and document structure. This evaluation adds code cross-reference analysis (rbac.go permissionMatrix) which reveals a spec-to-code contradiction not visible through document-only review.

---

## Findings

### [HIGH] apps/control-plane/internal/auth/rbac.go:39-60 — `read:metrics` permission absent from permissionMatrix

**Finding**: spec.md L137 maps `GET /metrics → read:metrics (admin only)`. The implementation path calls `auth.Authorize(ctx, "read:metrics")`. However, AUTH-001 rbac.go `permissionMatrix` defines the following permissions for `RoleAdmin`: `read:workflow, write:workflow, delete:workflow, read:recommendation, write:recommendation, read:audit, audit:read`. `read:metrics` is NOT included.

**Effect**: `EffectivePermissions([]Role{RoleAdmin})` returns a map without `read:metrics` → `Authorize(ctx, "read:metrics")` returns `ErrInsufficientPermission` for admin → HTTP 403. This contradicts:
- AC-AUTH2-Metrics-Admin case A (admin GET /metrics → expected HTTP 200)
- The stated security goal ("admin Bearer token 필요; 망분리 + 토큰 인증 이중 방어")

**Constraints in conflict**:
- spec.md §2.1: "apps/control-plane/internal/auth/rbac.go — 본 SPEC은 재사용만, 신규 정의 없음" (rbac.go NOT to be modified)
- REQ-AUTH2-001-E1: `GET /metrics → read:metrics (admin only)`
- rbac.go permissionMatrix: RoleAdmin does NOT have `read:metrics`

**Security classification**: Fail-closed (no unauthorized access path), but creates a self-defeating protection design — /metrics is inaccessible to ALL roles in production (AuthEnabled=true), while remaining accessible in dev/sandbox (AuthEnabled=false bypass). Prometheus scraping via admin Bearer token cannot work as specced.

**Required resolution**: One of:
a) Amend AUTH-001 rbac.go to add `read:metrics` to RoleAdmin (cross-SPEC change requiring AUTH-001 amendment)
b) Change authz approach for /metrics: role-check (`hasRole(user, "admin")`) instead of `Authorize(ctx, "read:metrics")`
c) Remove /metrics from the permission-protected table and use a separate access control mechanism

---

### [HIGH] apps/control-plane/internal/server/rest_handler.go:94-110 — /metrics handler absent from Mux()

**Finding**: AC-AUTH2-Metrics-Admin case A requires `GET /metrics → HTTP 200 + Prometheus exposition format`. Current `Mux()` registers only 4 routes: `POST /api/v1/workflows`, `GET /api/v1/workflows/{id}`, `GET /api/v1/workflows`, `GET /health`. No `/metrics` handler is registered. plan.md and spec.md §2.1 do not list rest_handler.go for modification in this SPEC.

**Effect**: Even if the read:metrics permission issue were resolved, admin GET /metrics would hit the authz middleware (pass RBAC), then reach the mux, which returns 405 Method Not Allowed (Go 1.22+ mux behavior for unregistered path patterns). AC-AUTH2-Metrics-Admin case A cannot pass.

**Note**: This is a standalone gap independent of the read:metrics permission issue above. Both must be resolved.

---

### [MINOR] .moai/specs/SPEC-AX-AUTH-001/acceptance.md:§6 — AC-AUTH-E2E-3 항목 미존재

**Finding**: SPEC-AX-AUTH-002 DoD L454 요구 사항: "AUTH-001 acceptance.md §6 AC-AUTH-E2E-3 status `SKIP → ACTIVE (by SPEC-AX-AUTH-002 S3)` 마커 추가". 그러나 AUTH-001 acceptance.md §6에 AC-AUTH-E2E-3 항목이 존재하지 않음 (grep: AC-AUTH-E2E-1 line 499, AC-AUTH-E2E-2 line 517만 존재). 해당 deliverable의 대상 항목이 없어 S3에서 "어디에 마커를 추가할 것인가"가 불분명하다.

**Effect**: auth_e2e_test.go의 t.Skip() 제거(AC-AUTH2-004-Sprint7-Unblock grep assertion)는 가능하지만, AUTH-001 acceptance.md 업데이트 deliverable은 정의가 불완전하다.

**Resolution**: AUTH-001 acceptance.md §6에 AC-AUTH-E2E-3 항목(TestE2E_Auth_RBACForbidden)을 신설 후 ACTIVE 마커 추가, 또는 DoD를 "auth_e2e_test.go SKIP 제거 확인만"으로 한정.

---

### [INFO] spec.md:L189 — D8 plan-auditor 잔여 발견 (cosmetic, confirmed minor)

plan-auditor D8 확인: "REQ-AUTH2-004-E3 정책 변경" orphan header가 §3.4 Event-driven section 내에 embedded. 구현 방해 없음. 향후 tooling 혼선 가능성만 존재. Run phase 이전 §7 Out of Scope bullet로 이동 권장 (non-blocking).

---

## Re-evaluation of Specific Focus Areas

### RBAC bypass attack vectors (privilege escalation, multi-role union edge)

**Privilege escalation via unknown scope**: `ParseRolesFromScope` uses `^iroum-ax:(admin|analyst|viewer)$` regex — unknown tokens silently dropped. `EffectivePermissions([])` → empty map → 403. **Correct.**

**Multi-role union**: analyst+viewer → union = analyst permissions (analyst is superset). No privilege escalation possible through union. `EffectivePermissions` is additive, Authorize checks membership in the union set. **Correct.**

**Keycloak scope manipulation**: out-of-scope per §7 (Keycloak admin responsibility). No spec defect.

### Default-deny safety net (mapping miss → 503)

`resolveRESTPermission(method, path) mapped=false → 503 + AUTHZ_MAPPING_MISSING audit`. Rationale (503 vs 401/403 avoids misleading signals) is sound. AC-AUTH2-001-2 (REST) and AC-AUTH2-001-5 (gRPC `codes.Unavailable`) are binary-testable. HEAD/OPTIONS bypass is covered by AC-AUTH2-001-3. **Design is correct; AC coverage is adequate.**

### AUTH-001 SKIP unblock — grep assertion 충분성

AC-AUTH2-004-Sprint7-Unblock: `grep -c "SPEC-AX-AUTH-002: RBAC REST handler wiring deferred" auth_e2e_test.go == 0`. This assertion is mechanically sufficient to verify the literal marker string is removed. Combined with AC-AUTH2-004-1 (viewer DELETE → 403 functional verification) and AC-AUTH2-004-2 (viewer GET → 200), the SKIP removal is adequately verified. **Sufficient.**

### In-flight race condition deferral (JWT 1h immutable rationale)

Exclusion #10 defers in-flight role change race to future SPEC. Justification: (a) JWT immutable claim — signed at issuance, (b) 1h expiry window per AUTH-001 NFR, (c) hot-revocation requires Keycloak Admin Events + back-channel logout (legitimate complexity justification). This is an industry-standard tradeoff for stateless JWT. **Justified deferral; rationale is sound.**

### Chain composition order verification

D7 fix: `chain.go.BuildRESTChain` and `BuildGRPCInterceptorChain` enforce order as pure functions. Unit tests `TestBuildRESTChain_Order` and `TestBuildGRPCInterceptorChain_Order` use record middleware pattern to verify call sequence. AC-AUTH2-003-6 (UNAUTHENTICATED before PermissionDenied as chain order proof) is a strong behavioral assertion. **Design is correct; verification is adequate.**

---

## Recommendations

1. **[Required, blocking]** `read:metrics` permission gap — Choose resolution path before Run phase:
   - **Option A (Recommended)**: Amend AUTH-001 rbac.go — add `"read:metrics"` to `RoleAdmin`'s permissions in `permissionMatrix`. This requires an AUTH-001 minor amendment (v0.1.2) and updates AUTH-001 acceptance.md's permissionMatrix assertion tests.
   - **Option B**: In authz.go's `resolveRESTPermission`, handle `/metrics` as a special case: instead of returning permission string `"read:metrics"`, return a new sentinel that triggers a direct role check (`hasRole(ctx, admin)`) bypassing `auth.Authorize()`. This avoids touching AUTH-001 but deviates from the uniform permission-based approach.
   - **Option C**: Remove `/metrics` from the SPEC-AX-AUTH-002 permission table (defer to SPEC-AX-AUTH-METRICS-001). Revise AC-AUTH2-Metrics-Admin to be out-of-scope. Simpler, keeps focus.

2. **[Required, blocking]** `/metrics` handler registration — Add to plan.md S1 deliverable: either implement `/metrics` handler in authz.go or add it to rest_handler.go Mux() with a Prometheus handler registration. Update §2.1 affected files accordingly.

3. **[Non-blocking, before S3]** AUTH-001 acceptance.md AC-AUTH-E2E-3 gap — Add AC-AUTH-E2E-3 stub entry to §6 of AUTH-001 acceptance.md so S3's deliverable ("SKIP → ACTIVE 마커 추가") has a target. Alternatively, redefine the DoD deliverable as "auth_e2e_test.go SKIP 제거 확인 + 기존 §6 performance table에 E2E-3 행 추가".

4. **[Non-blocking, cosmetic]** D8 spec.md L189 orphan header — Move to §7 Out of Scope before Run phase to eliminate grep/tooling confusion.

---

## Report File

`/home/sklee/moai/iroum-ax/.moai/reports/evaluator/SPEC-AX-AUTH-002-cross-validate.md`
