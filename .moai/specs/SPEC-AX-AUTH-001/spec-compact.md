# SPEC-AX-AUTH-001 — Compact (Run Phase Token Saver)

> 본 파일은 Run phase에서 토큰 절약을 위한 압축본. 상세는 spec.md / plan.md / acceptance.md / research.md 참조.

## Metadata

- id: SPEC-AX-AUTH-001
- version: 0.1.0
- status: draft
- domain: AX + AUTH (composite, 2 domains)
- harness: thorough
- methodology: TDD
- depends_on: SPEC-AX-001 (PASSED), SPEC-AX-CTRL-001 (PASSED)

## REQ Module Summary

| ID | Type | One-liner |
|----|------|-----------|
| REQ-AUTH-UBI-001 | Ubiquitous + State-driven (WHILE) | JWT sub → audit_logs.user_id propagation; AuthEnabled=false 시 cli-anonymous 폴백 유지; 거부 이벤트 AUTH_REJECTED 기록 |
| REQ-AUTH-001 | Event/State/Unwanted (E1, E2, S1, U1, U2) | RS256/EdDSA/ES256 JWT 검증, JWKS 1h TTL 캐시, 블랙리스트, HS256/none/future-iat 거부 |
| REQ-AUTH-002 | Event/Optional (E1, O1) | OIDC Discovery startup-time fetch, fail-fast on 10s timeout |
| REQ-AUTH-003 | Event/Unwanted (E1, E2, E3, U1) | gRPC Interceptor + REST middleware + FastAPI Depends; /health bypass; missing/malformed → 401 |
| REQ-AUTH-004 | State/Unwanted (S1, U1) | 3-role RBAC (admin/analyst/viewer) via scope claim union; 부족 → 403 + AUTH_FORBIDDEN |
| REQ-AUTH-005 | Event/Unwanted (E1, U1) | Logout 양 토큰 blacklist; Refresh token reuse → family invalidation |

## Affected Files (Top-level)

### Go (`apps/control-plane/`)
- `internal/auth/` (new dir): `verifier.go`, `jwks_cache.go`, `oidc_client.go`, `blacklist.go`, `rbac.go`, `context.go`, `refresh_family.go`, `errors.go`
- `internal/server/`: `grpc_interceptor.go` (new), `rest_middleware.go` (new), `grpc_server.go` (modify — ctx user_id 사용), `rest_handler.go` (modify + `/api/v1/auth/logout`)
- `internal/audit/`: `recorder.go` (resolveUserID 확장), `audit.go` (4 신규 Action 상수)
- `internal/config/config.go` (OIDC 필드 4 추가)
- `internal/scheduler/celery_envelope.go` (headers.user_id 추가, golden file 재생성)
- `cmd/server/main.go` (auth wiring)

### Python (`pipelines/`)
- `auth/` (new dir): `verifier.py`, `oidc_client.py`, `dependencies.py`, `blacklist.py`, `context.py`, `errors.py`
- `main.py` (modify — Depends(verify_token) 등록)
- `workers/{ingestion,generation,simulation}_worker.py` (modify — task context user_id 사용)
- `config/settings.py` (OIDC 필드 4 추가)

### Shared
- `pkg/auth/claims.{go,py}` (new)
- `schemas/openapi/openapi.yaml` (Bearer auth + 401/403 응답 명세 추가)

### Deployment
- `docker-compose.yml` (Keycloak)
- `deployments/helm/iroum-ax/templates/keycloak-{statefulset,realm-configmap}.yaml`

## AC Count Per REQ

| REQ | AC Count | DoD Reference |
|-----|---------|--------------|
| REQ-AUTH-UBI-001 | 4 (UBI-001-A/B/C/D) | acceptance.md §0 |
| REQ-AUTH-001 | 10 + 1 perf benchmark | §1 |
| REQ-AUTH-002 | 3 | §2 |
| REQ-AUTH-003 | 6 | §3 |
| REQ-AUTH-004 | 6 | §4 |
| REQ-AUTH-005 | 4 | §5 |
| E2E | 2 | §6 |
| **Total** | **36** | DoD §9 |

## Exclusions Count

15 entries (spec.md §5):
1. External OAuth providers (Google/Microsoft/GitHub/Apple — 망분리 위반)
2. MFA / TOTP / WebAuthn
3. 비밀번호 정책 (Keycloak 책임)
4. SAML 2.0 / WS-Federation
5. 전자정부 표준 인증 통합 → SPEC-AX-AUTH-EGOV-001 후속
6. 권한 위임 (Impersonation)
7. 세션 관리 콘솔 UI → SPEC-AX-CONSOLE-001
8. 감사 보고서 자동 생성
9. 토큰 암호화 (JWE) — 서명만 (JWS) 사용
10. 권한 캐싱 최적화
11. ABAC (Attribute-Based Access Control)
12. Workflow / Document level 권한 (RLS)
13. gRPC mTLS
14. OIDC RP-Initiated Logout / End-Session
15. JWT Token Revocation Endpoint (RFC 7009)

## Sprint Map (S0~S6)

- S0: Scaffolding + backward compat baseline
- S1: REQ-AUTH-001 (JWT verifier, JWKS cache, blacklist)
- S2: REQ-AUTH-002 (OIDC Discovery)
- S3: REQ-AUTH-003 (gRPC interceptor + REST middleware + FastAPI dep)
- S4: REQ-AUTH-004 (3-role RBAC + AUTH_FORBIDDEN audit)
- S5: REQ-AUTH-005 (Logout + refresh family invalidation)
- S6: E2E + Helm + docker-compose + Celery envelope golden file update

## Top 3 Architectural Decisions (User Review)

1. **OIDC Provider**: Keycloak 24.x LTS — 전자정부 표준 인증 vs Authentik vs Dex 거부. KEPCO E&C 전자정부 표준 강제 요구 시 SPEC-AX-AUTH-EGOV-001로 분리.
2. **Blacklist Storage**: Redis SET with EXPIREAT — Postgres revocation table vs in-memory 거부.
3. **Celery Worker user_id Propagation**: Envelope `headers.user_id` 헤더 + task_prerun signal handler — task signature 추가 거부.

## Open Questions (Sensible Defaults Exist)

1. 전자정부 표준 인증 요구 여부 (default: NO, Keycloak 진행)
2. 토큰 만료 정책 (default: access 1h / refresh 24h)
3. Refresh token rotation strictness (default: strict family invalidation)
4. Celery envelope golden file 업데이트 책임 (default: 본 SPEC S6에서)
5. Keycloak HA 도입 시점 (default: KEPCO E&C Go-Live 직전 별도 SPEC)

## Risk Top 3

- R-AUTH-001 JWKS unavailable → 1h TTL + stale-while-revalidate
- R-AUTH-002 Token leakage in logs → context에 raw token 저장 금지, log filter
- R-AUTH-007 Backward compat regression → S0 baseline + S3 명시 검증

## Performance Targets

- Token verify p99 < 5ms (JWKS cache hit)
- JWKS fetch p99 < 200ms (Keycloak local)
- RBAC scope parse + lookup p99 < 1ms
- Logout endpoint p99 < 50ms
- OIDC Discovery startup < 10s (fail-fast)

## DoD Highlights

- Coverage ≥ 85% (Go + Python)
- evaluator-active: S1/S4/S5 ≥ 0.80 (security-critical), 나머지 ≥ 0.75
- Token leakage: zap log + Python logging에 `Bearer ` 접두 0건
- Backward compat: AUTH_ENABLED=false 시 SPEC-AX-CTRL-001 모든 AC unchanged

## Schema Note

YAML frontmatter: `.claude/skills/moai/workflows/plan.md` Phase 2 8-field schema (id, version, status, created, updated, author, priority, issue_number) — SPEC-AX-001 / SPEC-AX-CTRL-001과 동일.
