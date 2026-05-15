# SPEC-AX-AUTH-002 Compact — Run Phase Loading

> 본 문서는 Run phase 로딩 토큰 절약용 압축본. 상세는 spec.md / plan.md / acceptance.md 참조.

---

## ID & Status

- SPEC-AX-AUTH-002 v0.1.0 draft (2026-05-15, ircp, high)
- Composite domain: AX + AUTH (도메인 카디널리티 2)
- Harness: thorough (security-critical)
- 의존: SPEC-AX-AUTH-001 v0.1.1 GREEN

## 목표 (1 문장)

AUTH-001 RBAC 라이브러리(`rbac.go` `Authorize`/`LogForbidden`)를 REST `Mux` + gRPC `UnaryServerInterceptor` 체인에 wiring하여, 인증 통과 후 핸들러 진입 전에 권한 결정을 강제하며 권한 부족 시 403/PermissionDenied + AUTH_FORBIDDEN audit를 기록한다.

## 5 REQ 모듈

1. **REQ-AUTH2-UBI-001**: 3 sub-clause — (a) 모든 deny 결정 `AUTH_FORBIDDEN` audit row, grant는 context annotation, (b) AuthEnabled=false 시 권한 체크 skip(백워드 호환), (c) 인증 후·핸들러 진입 전 사전 차단
2. **REQ-AUTH2-001 Mapping**: method+path → Permission lookup table (코드 명시, 운영 hot-reload 금지). REST 8 경로 + gRPC 3 RPC + bypass 4종(/health, /metrics, HEAD, OPTIONS). Default-deny 미정의 시 503 AUTHZ_MAPPING_MISSING
3. **REQ-AUTH2-002 REST Authz**: `auth.RESTMiddleware` 다음 `server.RESTAuthzMiddleware` chain. Authorize 호출, 실패 시 403 + WWW-Authenticate: insufficient_scope + LogForbidden audit
4. **REQ-AUTH2-003 gRPC Authz**: `grpc.ChainUnaryInterceptor(auth.UnaryServerInterceptor, server.UnaryAuthzInterceptor)`. 실패 시 `codes.PermissionDenied` + LogForbidden
5. **REQ-AUTH2-004 E2E**: AUTH-001 Sprint 7 `TestE2E_Auth_RBACForbidden` SKIP unblock. viewer DELETE → 403, analyst GET → 200, admin DELETE → 403 아님

## Method-Permission Mapping (canonical)

**REST**:
| Method | Path | Permission |
|--------|------|-----------|
| POST | /api/v1/workflows | write:workflow |
| GET | /api/v1/workflows/{id} | read:workflow |
| GET | /api/v1/workflows | read:workflow |
| DELETE | /api/v1/workflows/{id} | delete:workflow |
| POST | /api/v1/recommendations/{id}/feedback | write:recommendation |
| POST | /api/v1/documents/upload | write:workflow |
| GET | /health | bypass |
| GET | /metrics | bypass |
| HEAD/OPTIONS | * | bypass |

**gRPC**:
| FullMethod | Permission |
|-----------|-----------|
| /iroum.ax.v1.WorkflowService/CreateWorkflow | write:workflow |
| /iroum.ax.v1.WorkflowService/GetWorkflow | read:workflow |
| /iroum.ax.v1.WorkflowService/ListWorkflows | read:workflow |
| /grpc.health.v1.Health/Check | bypass |

## Sprint DAG

```
S0 (매핑 + 단위 테스트) → {S1 (REST middleware), S2 (gRPC interceptor)} → S3 (E2E unblock + cross-SPEC artifact)
```

## Affected Files

- 신규: `apps/control-plane/internal/server/authz.go` + `authz_test.go`
- 수정: `cmd/server/server.go` (chain wiring), `rest_handler.go` (Mux 변경 없음, chain은 server.go에서), `auth_e2e_test.go` (SKIP 제거), `schemas/openapi/openapi.yaml` (403/503 응답)
- 미수정: `rbac.go`, `middleware.go`, `grpc_server.go`, audit/actions.go

## Exclusions (9)

ABAC / 동적 매트릭스 / 권한 캐싱 / 위임 / 임시 권한 / 보고서 자동화 / 권한 propagation / per-resource ACL / row/cell-level

## 백워드 호환 invariant

AuthEnabled=false 시 SPEC-AX-001 / SPEC-AX-CTRL-001 / SPEC-AX-AUTH-001 모든 AC가 byte-identical 통과. `cli-anonymous` 폴백 보존.

## DoD highlights

- coverage ≥ 85%, golangci-lint 0 issue, `-race` 통과
- AUTH-001 `TestE2E_Auth_RBACForbidden` SKIP 제거 + GREEN
- AUTH-001 acceptance.md AC-AUTH-E2E-3 status 마커 업데이트
- MX tags: ANCHOR 1 + NOTE 1 + WARN 1 + REASON 모두 필수
- Total AC: 23

## Cross-SPEC Artifact 변경

- AUTH-001 `auth_e2e_test.go` `TestE2E_Auth_RBACForbidden`: SKIP 제거 (S3)
- AUTH-001 `acceptance.md` §6 AC-AUTH-E2E-3: `SKIP → ACTIVE (by SPEC-AX-AUTH-002 S3)` (S3)

## Open Questions (모두 sensible default 적용)

- /metrics: bypass (network ACL이 일차 방어)
- DELETE 핸들러: 후속 SPEC SPEC-AX-WF-DELETE-001
- Python FastAPI: 후속 SPEC SPEC-AX-AUTH-PY-001
- Default-deny 응답: 503 AUTHZ_MAPPING_MISSING
- Multi-role: union (analyst+viewer = 더 강한 권한)
