# SPEC-AX-AUTH-002 Compact — Run Phase Loading

> 본 문서는 Run phase 로딩 토큰 절약용 압축본. 상세는 spec.md / plan.md / acceptance.md 참조.

---

## ID & Status

- SPEC-AX-AUTH-002 v0.1.2 draft (2026-05-15, ircp, high) — iter 3 fixes applied (evaluator DISPUTE 대응)
- Composite domain: AX + AUTH (도메인 카디널리티 2)
- Harness: thorough (security-critical)
- 의존: SPEC-AX-AUTH-001 v0.1.1 GREEN, SPEC-AX-CTRL-001 부분 GREEN (server.go stub은 GAP)
- 후속: SPEC-AX-SERVER-001 (server bootstrap), SPEC-AX-WF-DELETE-001 (DELETE handler), **SPEC-AX-OBS-001 / SPEC-AX-METRICS-001 (/metrics endpoint — v0.1.2 Option C 분리)**

## 목표 (1 문장)

AUTH-001 RBAC 라이브러리(`rbac.go` `Authorize`/`LogForbidden`)를 REST/gRPC 미들웨어/인터셉터 + 체인 조합 헬퍼(`chain.go`)로 캡슐화하여, 인증 통과 후 핸들러 진입 전에 권한 결정을 강제하며 권한 부족 시 403/PermissionDenied + AUTH_FORBIDDEN audit를 기록한다. 실제 서버 bootstrap mount는 후속 SPEC `SPEC-AX-SERVER-001`이 chain.go 헬퍼를 한 줄로 호출하여 처리.

## 5 REQ 모듈

1. **REQ-AUTH2-UBI-001**: 3 sub-clause — (a) 모든 deny 결정 `AUTH_FORBIDDEN` audit row, grant는 context annotation, (b) AuthEnabled=false 시 권한 체크 skip(백워드 호환), (c) 인증 후·핸들러 진입 전 사전 차단
2. **REQ-AUTH2-001 Mapping**: method+path → Permission lookup table (코드 명시, 운영 hot-reload 금지). REST 7 경로 + gRPC 3 RPC + bypass 2종(/health, HEAD/OPTIONS). **`/metrics`는 본 SPEC 범위 외(v0.1.2 Option C 분리 — Exclusion #13)** — 후속 SPEC `SPEC-AX-OBS-001` 또는 `SPEC-AX-METRICS-001`. Default-deny 미정의 시 503 AUTHZ_MAPPING_MISSING. Ubiquitous `REQ-AUTH2-001-S1` (code-as-config) + Unwanted `REQ-AUTH2-001-U1` (default-deny) — D1 iter-2 fix로 ID 충돌 해소
3. **REQ-AUTH2-002 REST Authz**: `chain.go.BuildRESTChain`이 `auth.RESTMiddleware → server.RESTAuthzMiddleware → handler` 순서 강제 (D7 iter-2 fix). 실패 시 403 + WWW-Authenticate: insufficient_scope + LogForbidden audit
4. **REQ-AUTH2-003 gRPC Authz**: `chain.go.BuildGRPCInterceptorChain`이 `grpc.ChainUnaryInterceptor(auth.UnaryServerInterceptor, server.UnaryAuthzInterceptor)` 순서 강제 ServerOption 반환 (D7 iter-2 fix). 실패 시 `codes.PermissionDenied` + LogForbidden
5. **REQ-AUTH2-004 E2E**: AUTH-001 Sprint 7 `TestE2E_Auth_RBACForbidden` SKIP unblock. viewer DELETE → 403, viewer GET → 200 (D3 iter-2 default-deny 비적용 검증), Sprint7-Unblock grep 단언 (D5 iter-2). admin DELETE 통과 검증은 후속 SPEC SPEC-AX-WF-DELETE-001로 분리 (D3 iter-2 fix)

## Method-Permission Mapping (canonical, v0.1.2 fixed)

**REST**:
| Method | Path | Permission |
|--------|------|-----------|
| POST | /api/v1/workflows | write:workflow |
| GET | /api/v1/workflows/{id} | read:workflow |
| GET | /api/v1/workflows | read:workflow |
| DELETE | /api/v1/workflows/{id} | delete:workflow (admin only — RBAC 통과만 검증, 핸들러는 후속 SPEC) |
| POST | /api/v1/recommendations/{id}/feedback | write:recommendation |
| POST | /api/v1/documents/upload | write:workflow |
| GET | /health | bypass |
| HEAD/OPTIONS | * | bypass |

> v0.1.2 Note: `/metrics`는 본 SPEC 범위 외 (Option C — Exclusion #13). 후속 SPEC `SPEC-AX-OBS-001`/`SPEC-AX-METRICS-001`이 rbac.go permissionMatrix + rest_handler.go handler + authz mapping을 한 SPEC scope으로 처리. 임시 보호는 K8s NetworkPolicy / Helm values.

**gRPC**:
| FullMethod | Permission |
|-----------|-----------|
| /iroum.ax.v1.WorkflowService/CreateWorkflow | write:workflow |
| /iroum.ax.v1.WorkflowService/GetWorkflow | read:workflow |
| /iroum.ax.v1.WorkflowService/ListWorkflows | read:workflow |
| /grpc.health.v1.Health/Check | bypass |

## Sprint DAG

```
S0 (매핑 + chain.go 헬퍼 + 단위 테스트 + chain order 검증) → {S1 (REST middleware), S2 (gRPC interceptor)} → S3 (E2E unblock + cross-SPEC artifact + Sprint7-Unblock grep 단언)
```

## Affected Files (v0.1.2 fixed)

- 신규: `apps/control-plane/internal/server/authz.go` + `authz_test.go` + `apps/control-plane/internal/server/chain.go` (D2 iter-2 fix — 체인 조합 헬퍼)
- 수정: `auth_e2e_test.go` (SKIP 제거), `schemas/openapi/openapi.yaml` (403/503 응답)
- 미수정: `rbac.go`, `middleware.go`, `grpc_server.go`, `rest_handler.go`, `audit/actions.go`, `cmd/server/server.go` (40-line stub, 본 SPEC 범위 외 — 후속 SPEC SPEC-AX-SERVER-001 책임)

## Exclusions (13 — v0.1.2 expanded from 12)

1. ABAC / 2. 동적 매트릭스 / 3. 권한 캐싱 / 4. 위임 / 5. 임시 권한 / 6. 보고서 자동화 / 7. 권한 propagation / 8. per-resource ACL / 9. row/cell-level / 10. In-flight 권한 변경 race (D6) / 11. DELETE REST 핸들러 (D3) / 12. cmd/server/server.go 부트스트랩 (D2) / **13. Prometheus `/metrics` endpoint (v0.1.2 Option C — 후속 SPEC SPEC-AX-OBS-001/SPEC-AX-METRICS-001)**

## 백워드 호환 invariant

AuthEnabled=false 시 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-AUTH-001 모든 AC가 byte-identical 통과. `cli-anonymous` 폴백 보존. `chain.go.BuildRESTChain/BuildGRPCInterceptorChain`가 `authEnabled=false`이면 미들웨어/인터셉터 chain에서 제외.

## DoD highlights

- coverage ≥ 85%, golangci-lint 0 issue, `-race` 통과
- AUTH-001 `TestE2E_Auth_RBACForbidden` SKIP 제거 + GREEN
- AC-AUTH2-004-Sprint7-Unblock grep 단언 PASS (D5 iter-2)
- AUTH-001 `acceptance.md` AC-AUTH-E2E-3 status 마커 업데이트는 **본 SPEC 범위 외 별도 chore commit** (v0.1.2 cross-ref Minor)
- D7 chain order 단위 테스트 (`TestBuildRESTChain_Order` + `TestBuildGRPCInterceptorChain_Order`) GREEN
- MX tags: ANCHOR 2 (authz.go + chain.go) + NOTE 2 + WARN 1 + REASON 모두 필수
- **Total AC: 22** (v0.1.1 25 → AC-AUTH2-Metrics-Admin 3 sub-cases 삭제)

## Cross-SPEC Artifact 변경

- AUTH-001 `auth_e2e_test.go` `TestE2E_Auth_RBACForbidden`: SKIP 제거 (S3 deliverable) + grep 단언 (D5)
- AUTH-001 `acceptance.md` §6 AC-AUTH-E2E-3: `SKIP → ACTIVE (by SPEC-AX-AUTH-002 S3)` — **별도 chore commit** (v0.1.2 cross-ref Minor)
- 후속 SPEC SPEC-AX-OBS-001/SPEC-AX-METRICS-001: `/metrics` permission matrix + handler + authz mapping (v0.1.2 Option C)

## v0.1.2 (iter 3) Defect Fixes Applied

- **Option C — /metrics 분리**: REST mapping table에서 /metrics 행 제거, Exclusion #13 신설, AC-AUTH2-Metrics-Admin (3 sub-cases) 삭제로 Total AC 25 → 22, plan.md S1에서 /metrics 작업 제거. 사유: rbac.go permissionMatrix `read:metrics` 미정의 + rest_handler.go `/metrics` handler 미등록 → cross-SPEC 변경 필요로 명세-코드 모순. 후속 SPEC SPEC-AX-OBS-001 또는 SPEC-AX-METRICS-001.
- **D8 cosmetic**: spec.md L189 orphan `REQ-AUTH2-004-E3 정책 변경` narrative header를 §3.5 마무리 단락으로 정리하여 REQ-AUTH2-004-E2 (viewer GET) 정의 명시적 통합.
- **AUTH-001 cross-ref Minor**: AUTH-001 `acceptance.md` §6 AC-AUTH-E2E-3 마커 업데이트는 본 SPEC 범위 외 별도 chore commit으로 분리. plan.md S3 deliverable + §6 Cross-SPEC Impact 테이블에 명시.

## Open Questions (모두 sensible default 적용)

- /metrics: **DEFERRED to SPEC-AX-OBS-001** (v0.1.2 Option C)
- DELETE 핸들러: 후속 SPEC SPEC-AX-WF-DELETE-001
- Server bootstrap: 후속 SPEC SPEC-AX-SERVER-001 (D2 iter-2 fix)
- Python FastAPI: 후속 SPEC SPEC-AX-AUTH-PY-001
- Default-deny 응답: 503 AUTHZ_MAPPING_MISSING
- Multi-role: union (analyst+viewer = 더 강한 권한)
- In-flight 권한 변경: Exclusion §10 (JWT immutable + 만료 1h)
