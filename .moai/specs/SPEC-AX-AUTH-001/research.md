# SPEC-AX-AUTH-001 Research Notes

> Phase: Plan (Research sub-phase)
> Author: manager-spec (Opus 4.7, extended reasoning applied to 5 architectural decisions)
> Created: 2026-05-14

본 문서는 SPEC-AX-AUTH-001 spec.md / plan.md 작성을 뒷받침하는 외부 참조 자료, 의사결정 근거, 한국 공공기관 규제 컨텍스트, OIDC provider 비교 분석, 그리고 risk register 상세 분석을 담는다.

---

## 1. Reference Implementations

### 1.1 Keycloak 24.x LTS (OIDC Provider)

- 출처: https://www.keycloak.org/
- 버전: 24.x LTS (2026 기준 stable major)
- 핵심 패턴:
  - Realm 단위 격리 (멀티테넌트 준비 — 본 SPEC은 single realm `iroum-ax`)
  - OIDC Discovery: `http://<keycloak>/realms/iroum-ax/.well-known/openid-configuration`
  - JWKS: `/realms/iroum-ax/protocol/openid-connect/certs`
  - Token endpoint: `/realms/iroum-ax/protocol/openid-connect/token`
  - 클라이언트 타입: Public (PKCE), Confidential (client secret)
  - Scope 매�ping: Realm Roles → Client Scopes → JWT `scope` claim
- 본 SPEC에서의 활용:
  - K8s StatefulSet으로 단일 인스턴스 배포 (PoC), 후속 SPEC에서 3-replica HA
  - Realm 사전 정의: ConfigMap에 realm.json mount, Keycloak 시작 시 import
  - 클라이언트: `iroum-ax-cli` (public + PKCE) — Console UI에서 사용 예정
- Korean 운영 사례: 한국전자통신연구원(ETRI), 다수 핀테크, 공공기관 SSO 도입 사례 다수.

### 1.2 golang-jwt/jwt v5 (Go JWT 라이브러리)

- 출처: https://github.com/golang-jwt/jwt
- 버전: v5.2.x (2026 stable)
- 핵심 패턴:
  - `jwt.ParseWithClaims(tokenString, &Claims{}, keyFunc, jwt.WithAlgorithms("RS256", "EdDSA", "ES256"))` — algorithm allow-list 강제
  - `jwt.WithLeeway(30 * time.Second)` — clock skew 허용
  - `jwt.WithAudience("iroum-ax-control-plane")` — aud 자동 검증
  - `keyFunc`: JWKS cache에서 kid에 매칭되는 RSA/EC public key 조회
- 본 SPEC `verifier.go`가 v5 API 활용.
- Context7 라이브러리 ID: `/golang-jwt/jwt`

### 1.3 coreos/go-oidc v3 (OIDC Discovery + JWKS Client)

- 출처: https://github.com/coreos/go-oidc
- 버전: v3.10.x
- 핵심 패턴:
  - `oidc.NewProvider(ctx, issuerURL)` — Discovery 자동 수행
  - `provider.NewRemoteKeySet(ctx, jwksURL)` — JWKS 캐시 자동 관리 (Cache-Control 헤더 + 5분 fallback TTL)
  - `provider.Verifier(&oidc.Config{ClientID: audience})` — JWT 검증기 wrapping
- 본 SPEC에서의 활용:
  - go-oidc의 RemoteKeySet은 자체 캐싱하므로 별도 `jwks_cache.go` 구현 vs 활용 vs wrapping 결정 필요
  - **결정**: go-oidc의 RemoteKeySet을 wrapping하되, **자체 TTL 정책(1시간 hard TTL + stale-while-revalidate)을 superset으로 구현**. 이유: go-oidc는 Cache-Control 헤더 의존인데 Keycloak의 default Cache-Control이 짧을 수 있어 1h hard floor 필요.
- Context7 라이브러리 ID: `/coreos/go-oidc`

### 1.4 python-jose (Python JWT 라이브러리)

- 출처: https://github.com/mpdavis/python-jose
- 버전: v3.3.0 (cryptography backend)
- 핵심 패턴:
  - `jose.jwt.decode(token, key, algorithms=["RS256", "EdDSA"], audience="iroum-ax-pipelines")`
  - JWKS fetch: 별도 `httpx` 사용 (python-jose는 JWKS 클라이언트 미포함)
- 대안: `PyJWT 2.8` — 라이브러리 활성도가 더 높음. **결정: PyJWT 채택 — 활성도 + 보안 패치 빈도**.
- Context7 라이브러리 ID: `/jpadilla/pyjwt`

### 1.5 oauth2-proxy (Reference Implementation Pattern)

- 출처: https://github.com/oauth2-proxy/oauth2-proxy
- 본 SPEC 직접 의존 없음, but 참조: gRPC + REST 동시 보호 패턴, scope 기반 RBAC 매트릭스 구조
- 특히 ALLOWED_GROUPS / ALLOWED_EMAILS 패턴 → 본 SPEC의 scope allow-list 매트릭스에 영감

### 1.6 charmbracelet/crush (Go Service Patterns)

- 출처: `.claude/rules/moai/core/lsp-client.md`
- 본 SPEC에서의 활용:
  - context propagation 패턴 (auth user_id을 context에 주입)
  - graceful shutdown (JWKS background refresh goroutine 종료 처리)

---

## 2. OIDC Provider Comparison (Top 3 — User Review Required)

본 SPEC의 가장 중요한 architectural decision. Extended reasoning 적용.

### 2.1 Comparison Matrix

| Criterion | Keycloak 24.x LTS | 전자정부 표준 인증 | Authentik | Dex |
|-----------|-------------------|------------------|-----------|-----|
| 자체 호스팅 | ✓ (StatefulSet) | (정부 인프라) | ✓ | ✓ |
| 망분리 정합 | ✓ | △ (정부망 통합 필요) | ✓ | ✓ |
| OIDC 지원 | ✓ standard | △ (proprietary protocol) | ✓ standard | ✓ standard |
| SAML 지원 | ✓ (후속 필요 시) | ✓ | ✗ | ✗ |
| Helm Chart 공식 | ✓ codecentric/keycloak | ✗ | ✓ | ✓ |
| Admin UI | ✓ 풍부 | ✓ | ✓ | △ |
| 한국 운영 사례 | 다수 | 공공기관 표준 | 적음 | 적음 |
| 통합 복잡도 | 중 | 높음 (인증서 발급, 정부 인증) | 중 | 중 |
| 라이센스 | Apache 2.0 | 정부 표준 (제약) | MIT | Apache 2.0 |
| 활성도 (2026) | 매우 활발 | 정부 주도 | 활발 | 활발 |
| Local dev 편의 | docker-compose 1줄 | 불가 (정부망 필수) | docker-compose | docker-compose |

### 2.2 Decision: Keycloak 24.x LTS

**근거**:
1. **검증된 자체 호스팅**: K8s StatefulSet + Postgres backing store가 production-ready. PoC 단계 단일 인스턴스로 시작, 운영 단계 HA 확장 path 명확.
2. **망분리 정합**: 외부 OAuth federation 없이도 standalone OIDC provider로 동작.
3. **한국 운영 사례 다수**: ETRI, 다수 핀테크, 공공기관 도입 사례. 운영 노하우 확보 용이.
4. **Local dev 편의**: docker-compose에 1개 컨테이너 추가만으로 PoC 가능. 전자정부 표준 인증은 local dev 불가.
5. **확장성**: 향후 SAML 통합, federation, MFA 모두 Keycloak 단에서 옵션으로 활성 가능 (별도 코드 변경 0건).

**거부 사유**:
- **전자정부 표준 인증**: 통합 복잡도가 매우 높음 (정부 인증서 발급, 정부망 통합 절차, dev 환경 불가). 본 SPEC scope를 벗어남. 후속 SPEC `SPEC-AX-AUTH-EGOV-001`로 분리 권장. KEPCO E&C가 명시적으로 요구하는 경우에만 진행.
- **Authentik**: 신생, 한국 운영 사례 적음. 검증 위험.
- **Dex**: 외부 IdP federation에 강점이 있으나 standalone OIDC provider로는 Keycloak이 더 풍부함.

### 2.3 사용자 검토 요청 사항 (Open Questions)

1. **KEPCO E&C가 전자정부 표준 인증 사용을 명시적으로 요구하는가?**
   - YES → 본 SPEC을 중단하고 `SPEC-AX-AUTH-EGOV-001`로 분리하여 신규 SPEC 작성
   - NO → 본 SPEC Keycloak 진행

2. **Keycloak admin 콘솔 한국어 UI 충분한가?**
   - Keycloak admin UI는 다국어 지원이지만 일부 한국어 번역 미완. 운영팀이 영어 가능 시 OK.

---

## 3. Korean Public Sector Regulatory Constraints

### 3.1 망분리 (Network Segmentation) 정합성

`tech.md` §9.1 + SPEC-AX-001 REQ-UBI-001 데이터 주권:
- Keycloak은 동일 K8s namespace에 배포. 외부 인터넷 fetch 0건.
- OIDC Discovery URL = K8s service DNS (`http://keycloak.iroum-ax.svc.cluster.local:8080/realms/iroum-ax`)
- JWKS endpoint도 internal service URL
- 외부 federation (Google / Microsoft / Apple ID)은 §5 Exclusion #1로 명시 거부

### 3.2 PISA (기관 정보보호 수준 진단) 준수

`tech.md` §9.4:
- 접근 제어 RBAC: REQ-AUTH-004 (3-role)
- 암호화 — 전송: TLS 1.2+ (K8s Ingress 단에서 처리, 본 SPEC 외)
- 암호화 — JWT 서명: RS256 / EdDSA / ES256 비대칭키만 허용 (REQ-AUTH-001-U1)
- 감사: 모든 인증 이벤트 audit_logs 기록 (`AUTH_REJECTED`, `AUTH_FORBIDDEN`, `AUTH_LOGOUT`, `AUTH_REFRESH_REUSE_DETECTED` 4종 신규 액션)
- 백업: Keycloak Postgres backing store 일일 백업 (운영 단계 후속 SPEC)

### 3.3 PIPA (개인정보보호법) 준수

- 사용자 식별자: JWT `sub` claim = Keycloak user UUID (개인정보 직접 포함 안 함)
- 이름/이메일은 Keycloak DB에만 저장, iroum-ax DB는 sub UUID만 보관 → 정보 최소화 원칙
- audit_logs에 작업 추적 가능한 sub UUID 저장 (감사원 요구 시 Keycloak DB와 join하여 실명 도출 가능)

### 3.4 감사원 추적성

`product.md` §6.1 KEPCO E&C 운영 배포 prerequisite:
- 모든 audit_logs row의 user_id가 실 사용자 식별 가능해야 함
- 본 SPEC: REQ-AUTH-UBI-001로 정확히 충족
- 인증 거부 / 권한 부족도 추적 (`AUTH_REJECTED`, `AUTH_FORBIDDEN`) — 감사원의 "차단된 접근 시도" 항목에 대응

---

## 4. Architectural Decision Log (Top 3 — User Review Required)

### Decision 1: OIDC Provider — Keycloak (vs 전자정부 표준 vs Authentik vs Dex)

상세는 §2.2 참조. 본 SPEC 채택: **Keycloak 24.x LTS**.

**사용자 검토 요청 사항**: KEPCO E&C가 전자정부 표준 인증을 강제하는가? PoC 단계에서는 Keycloak 진행하되, 운영 단계에서 전자정부 표준 인증 통합 SPEC을 언제 만들지 결정 필요.

### Decision 2: Token Blacklist Storage — Redis (vs Postgres revocation table)

- 옵션 A: Redis SET with EXPIREAT (본 SPEC 채택)
- 옵션 B: Postgres `revoked_tokens` 테이블
- 옵션 C: In-memory (Go process local) — 거부 (분산 환경 불일치)

**선택: A (Redis)**

**근거**:
- Latency: < 5ms (vs Postgres < 50ms)
- TTL 자연 처리: token.exp - now() = Redis EXPIREAT. Postgres는 별도 cleanup job 필요.
- 기존 인프라 활용: SPEC-AX-CTRL-001가 Redis를 dispatch broker로 사용. 추가 의존성 0건.
- Cross-language consistency: Go와 Python이 동일 key prefix `auth:blacklist:` 공유.

**단점**:
- Redis 데이터 손실 시 blacklist 손실 (단, 토큰 만료 시각이 짧으므로 attack window 제한적)
- 운영 단계 후속 SPEC에서 AOF persistence + Redis HA 도입 가능

**사용자 검토 요청 사항**: 토큰 만료 정책 (access 1h / refresh 24h)이 적절한가? Keycloak realm 설정에서 조정 가능.

### Decision 3: Python Celery Worker user_id Propagation — Envelope Header (vs Argument)

- 옵션 A: Celery envelope `headers.user_id` 헤더 (본 SPEC 채택)
- 옵션 B: Task 함수 시그니처에 user_id 인자 추가

**선택: A (Envelope header)**

**근거**:
- Backward compatibility: 기존 task signature 보존. SPEC-AX-001 Python pipelines 변경 최소화.
- Celery 패턴: `task_prerun` signal handler가 message.headers를 읽어 task-local contextvar에 주입. 표준 패턴.
- AC-AUTH-E2E-1: envelope `headers.user_id` propagation 검증.

**단점**:
- Celery envelope golden file 재생성 필요 (SPEC-AX-CTRL-001 `celery_envelope_v2.json` 업데이트). S6에서 처리.
- Header 누락 시 fallback 처리 필요 (envelope header 없으면 cli-anonymous).

**사용자 검토 요청 사항**: SPEC-AX-CTRL-001의 골든 파일을 본 SPEC에서 업데이트하는 것에 동의하는가? (Cross-SPEC artifact 변경)

---

## 5. JWT Library Selection Detail

### 5.1 Go: golang-jwt/jwt v5

- 검토: lestrrat-go/jwx (대안), gopkg.in/square/go-jose (deprecated)
- **선정: golang-jwt/jwt v5**
- 이유:
  - v5 in 2026 = stable + breaking change 적음
  - allow-list `jwt.WithAlgorithms(...)` 강제 — Algorithm Confusion Attack 자동 방어
  - `jwt.WithLeeway` clock skew — 본 SPEC NFR 자연 충족
  - golang-jwt 조직의 활성도 + 보안 패치 빈도 우수

### 5.2 Python: PyJWT 2.8 (vs python-jose)

| 항목 | PyJWT 2.8 | python-jose 3.3 |
|------|-----------|----------------|
| 활성도 (2026) | 매우 활발 | 보통 |
| 보안 패치 빈도 | 빠름 | 느림 |
| API 직관성 | 단순 | OOP 스타일 |
| JWKS 클라이언트 내장 | ✓ `PyJWKClient` | ✗ (별도 httpx 필요) |
| algorithm allow-list | ✓ `algorithms=[...]` | ✓ |

**결정: PyJWT 2.8**.

**근거**: 보안 패치 빈도가 결정적 + `PyJWKClient` 내장 + 단순한 API.

---

## 6. Risk Register Detail

### R-AUTH-001 — JWKS Endpoint Unavailability

**Likelihood**: Medium (Keycloak 단일 인스턴스, 재시작 / 네트워크 partition)
**Impact**: High (모든 인증 요청 실패 → 서비스 중단)
**Detection**: AC-AUTH-001-9 (JWKS unavailable 시 cached keys 사용)
**Mitigation**:
- 1h hard TTL JWKS cache (REQ-AUTH-001-E2)
- Stale-while-revalidate: cache 만료 후 fetch 실패 시 cached keys 계속 사용 (degraded mode)
- HTTP 503 + `Retry-After: 30` 헤더 (cache 자체가 없는 startup 직후만)
- 운영 단계 후속 SPEC: Keycloak HA (3-replica StatefulSet)
**Residual Risk**: Low (PoC 단계에서 수용)

### R-AUTH-002 — Token Leakage in Logs

**Likelihood**: Medium (개발자가 무심코 `logger.Debug("auth header:", token)` 작성)
**Impact**: High (토큰 만료 전까지 impersonation 가능)
**Detection**:
- 로그 스캐닝 정규식 `Bearer [A-Za-z0-9-_\.]{20,}` 검출 시 PR 자동 reject (후속 CI 룰)
- 본 SPEC 종료 시 zap log + Python logging에 `Bearer ` 접두 0건 확인 (DoD §9 마지막 항목)
**Mitigation**:
- `auth/context.go`에 raw token을 context에 저장 안 함 — 검증 후 즉시 폐기, claims만 보관
- zap log 필드에 `Authorization` 헤더 직접 출력 금지 — `request_id`만 출력
- Python logging filter에 token redaction 추가
- Code review checklist 항목: "Authorization 헤더가 log 출력되지 않는가?"
**Residual Risk**: Low

### R-AUTH-003 — Clock Skew Across Services

**Likelihood**: Medium (K8s pod NTP 미동기 가능)
**Impact**: Medium (정상 토큰이 만료 / 미래 iat로 거부)
**Detection**: AC-AUTH-001-7 (clock skew within 30s 허용 검증)
**Mitigation**:
- REQ-AUTH-001-E1: ±30초 clock skew 허용 (OAuth 2.0 BCP)
- 운영 환경 NTP 동기 docs (`docs/deployment.md` 후속 SPEC에서 추가)
- K8s node-level chrony / systemd-timesyncd 활성
**Residual Risk**: Low

### R-AUTH-004 — Refresh Token Family Invalidation Race Condition

**Likelihood**: Low (정상 시나리오 발생 안 함, 공격 시도 시 발생)
**Impact**: Medium (정상 사용자가 family invalidation 경험 가능)
**Detection**: AC-AUTH-005-3 (refresh reuse detection)
**Mitigation**:
- Redis Lua script로 atomic check-and-blacklist
- 사용자 friendly 에러 메시지: "다른 기기에서 로그인 감지됨, 재로그인 필요"
- Lua script eval failure 시 fallback: 보수적으로 family 전체 blacklist (false positive 수용)
**Residual Risk**: Low

### R-AUTH-005 — Keycloak Single Point of Failure (PoC)

**Likelihood**: Medium (단일 인스턴스 운영)
**Impact**: High (Keycloak down 시 신규 로그인 불가)
**Detection**: Keycloak readiness probe + monitoring
**Mitigation**:
- 본 SPEC: 단일 인스턴스 수용 (PoC 단계)
- 운영 단계 후속 SPEC: Keycloak HA (3-replica + Postgres backing store + Infinispan distributed cache)
- 기존 토큰은 JWKS 캐시로 검증 가능하므로 Keycloak 다운 시에도 인증 통과 (신규 로그인만 불가, 운영 지속성 부분 확보)
**Residual Risk**: Medium (수용 risk, 운영 monitoring 후속)

### R-AUTH-006 — Algorithm Confusion Attack (HS256 / none)

**Likelihood**: Low (라이브러리가 잘 처리하지만 미스컨피그 시)
**Impact**: Critical (forged token으로 인증 우회)
**Detection**:
- AC-AUTH-001-5 (HS256 rejection)
- AC-AUTH-001-6 (none alg rejection)
**Mitigation**:
- REQ-AUTH-001-U1: allow-list `[RS256, EdDSA, ES256]`만 명시적으로 허용
- `jwt.WithAlgorithms(...)` 강제 (Go)
- `algorithms=[...]` 인자 강제 (Python)
- JWKS 응답의 `alg` 필드와 token `alg` cross-check
**Residual Risk**: Very Low

### R-AUTH-007 — Backward Compatibility Regression

**Likelihood**: Medium (resolveUserID 로직 확장 시 기존 cli-anonymous 폴백 깨질 가능)
**Impact**: High (SPEC-AX-001 / SPEC-AX-CTRL-001 AC 회귀 → 본 SPEC도 차단)
**Detection**:
- `tests/integration/test_auth_backward_compat.py` (S0에서 baseline 작성)
- S3 완료 시 AC-CTRL-UBI-002-A/B/C가 unchanged 결과 반환 확인
- AC-AUTH-UBI-001-C + AC-AUTH-E2E-2가 byte-identical 검증
**Mitigation**:
- `resolveUserID(ctx, providedUserID, authEnabled)`: authEnabled=false 시 무조건 cli-anonymous 반환 (기존 동작 보존)
- S0에서 regression baseline 테스트 미리 작성
- CI에서 본 SPEC 진행 중에도 SPEC-AX-CTRL-001 전체 테스트 통과 유지 의무
**Residual Risk**: Low (자동화 보장 + 명시적 regression guard)

---

## 7. Cognitive Bias Check

- **Anchoring bias**: SPEC-AX-CTRL-001 Sprint Contract Exclusion §2가 "Authentication / SSO / JWT" 명시 보류 → 본 SPEC에서 한 번에 SSO + JWT + RBAC + Refresh + Logout 모두 다룰 위험. **Mitigation**: §5 Exclusion 15개 항목으로 명시적으로 범위 제한 (MFA, SAML, 전자정부, ABAC, mTLS, etc.).
- **Confirmation bias**: Keycloak이 익숙해서 검토 없이 선택할 위험 → §2.1 4-way matrix로 명시 평가, §2.3에서 KEPCO E&C 전자정부 표준 요구 여부 확인 질문 명시.
- **Sunk cost bias**: 기존 `cli-anonymous` 폴백 코드를 모두 제거하려는 충동 → 백워드 호환성 invariant로 명시 유지 (REQ-AUTH-UBI-001 WHILE clause). R-AUTH-007 regression guard로 자동화 보장.
- **Overconfidence**: "Keycloak이 production-grade이니 PoC 단일 인스턴스로 충분" → R-AUTH-005 Medium risk로 명시 수용, 운영 단계 HA SPEC 분리 path 명시.

**This option might fail because**:
- (a) Keycloak이 K8s 단일 인스턴스에서 메모리/CPU 요구사항이 예상보다 큼 → mitigation: PoC 단계 resource request/limit 측정, 운영 단계 HA SPEC에서 재산정
- (b) Python Celery envelope golden file 재생성이 SPEC-AX-CTRL-001 cross-SPEC artifact 변경 → mitigation: S6에서 envelope golden file 신중하게 업데이트, byte-for-byte 비교 테스트 보존, SPEC-AX-CTRL-001 AC-CTRL-005-1 갱신 필요 (cross-SPEC handoff note)
- (c) RBAC 3-role이 KEPCO E&C 운영 요구사항을 충족 못할 가능성 → mitigation: 5.PRE-DEPLOY 검토에서 운영팀과 role 매트릭스 검증, 필요 시 본 SPEC 진행 중 RBAC 확장 가능 (역할 추가는 backward compatible)

---

## 8. References (External)

| Source | Topic | URL/Path |
|--------|-------|----------|
| `tech.md` §9.1, §9.4 | 망분리, PISA, 감사 로그 | `.moai/project/tech.md` |
| `product.md` §4, §6.1 | 페르소나, KEPCO E&C 운영 prerequisite | `.moai/project/product.md` |
| `structure.md` §3.2 | JWT TBD 항목 (본 SPEC이 해소) | `.moai/project/structure.md` |
| SPEC-AX-001 §3.1 REQ-UBI-003 | cli-anonymous fallback | `.moai/specs/SPEC-AX-001/spec.md` |
| SPEC-AX-CTRL-001 §3.1, §5 Exclusion #2 | 인증 deferral | `.moai/specs/SPEC-AX-CTRL-001/spec.md` |
| SPEC-AX-CTRL-001 AC-CTRL-UBI-002-A/B/C | regression baseline target | `.moai/specs/SPEC-AX-CTRL-001/acceptance.md` |
| Keycloak Docs (24.x LTS) | OIDC provider | https://www.keycloak.org/docs/latest/ |
| golang-jwt/jwt v5 | Go JWT 라이브러리 | https://github.com/golang-jwt/jwt |
| coreos/go-oidc v3 | OIDC client | https://github.com/coreos/go-oidc |
| PyJWT 2.8 | Python JWT 라이브러리 | https://github.com/jpadilla/pyjwt |
| OAuth 2.0 BCP (RFC 8252, 9700) | Security best practices | https://datatracker.ietf.org/doc/html/rfc9700 |
| OWASP JWT Cheat Sheet | JWT security checklist | https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html |
| RFC 7519 (JWT) | JWT standard | https://datatracker.ietf.org/doc/html/rfc7519 |
| RFC 8252 (OAuth2 for Native Apps) | PKCE recommendation | https://datatracker.ietf.org/doc/html/rfc8252 |
| oauth2-proxy | Reference pattern | https://github.com/oauth2-proxy/oauth2-proxy |

---

## 9. Open Questions (Phase 2 RED 진입 전 orchestrator 확인 필요)

> [HARD] manager-spec(본 agent)은 AskUserQuestion 호출 금지. 아래 질문은 orchestrator가 AskUserQuestion으로 사용자에게 확인한다.
> 각 질문에 sensible default가 존재하므로 응답 없이도 Phase 2 RED 진입 가능 (Ready=YES).

### Q1: 전자정부 표준 인증 사용 요구 여부

- 질문: KEPCO E&C 또는 후속 고객사가 전자정부 표준 인증 사용을 명시적으로 요구하는가?
- 옵션 A (권장 default): NO → 본 SPEC Keycloak 진행, 전자정부 통합은 후속 SPEC `SPEC-AX-AUTH-EGOV-001`로 분리
- 옵션 B: YES → 본 SPEC 중단, `SPEC-AX-AUTH-EGOV-001` 우선 작성

### Q2: 토큰 만료 정책

- 질문: Access token / Refresh token 만료 시간을 어떻게 설정할 것인가?
- 옵션 A (권장 default): Access 1h / Refresh 24h (OAuth 2.0 BCP 권장)
- 옵션 B: Access 15m / Refresh 8h (높은 보안 요구 시)
- 옵션 C: Access 8h / Refresh 7d (사용자 편의 우선)
- 본 SPEC은 옵션 A를 default로 진행, Keycloak realm 설정에서 후 조정 가능

### Q3: Refresh Token Rotation 강도

- 질문: Refresh token rotation 시 family invalidation을 strict하게 적용할 것인가?
- 옵션 A (권장 default): YES → AC-AUTH-005-3 strict mode (재사용 1회 감지 시 family 전체 invalidation)
- 옵션 B: NO → 재사용 감지 시 해당 토큰만 거부, family는 유지 (사용자 편의 우선)

### Q4: Celery Envelope Golden File 업데이트 책임

- 질문: SPEC-AX-CTRL-001의 `celery_envelope_v2.json` 골든 파일을 본 SPEC S6에서 업데이트할 것인가?
- 옵션 A (권장 default): YES → 본 SPEC S6에서 업데이트 + SPEC-AX-CTRL-001 AC-CTRL-005-1 회귀 검증
- 옵션 B: NO → SPEC-AX-CTRL-001 maintenance SPEC 별도 작성 후 본 SPEC 진행

### Q5: Keycloak HA 도입 시점

- 질문: PoC 단계 Keycloak 단일 인스턴스 수용 (R-AUTH-005 residual Medium) — 운영 단계 HA 도입 시점은?
- 옵션 A (권장 default): Anchor 고객(KEPCO E&C) Go-Live 직전 별도 SPEC `SPEC-AX-AUTH-HA-001` 신규 작성
- 옵션 B: 본 SPEC에 HA 포함 (scope 대폭 증가)

---

## 10. Definition of Done (Research Phase)

- [x] 6개 reference implementation 문서화 (Keycloak, golang-jwt/jwt v5, coreos/go-oidc v3, PyJWT, oauth2-proxy, charmbracelet/crush)
- [x] OIDC provider 4-way comparison matrix + decision (Keycloak)
- [x] Korean public sector regulatory constraints (망분리, PISA, PIPA, 감사원 추적성) 매핑
- [x] Top 3 architectural decisions trade-off 기록 (Provider, Blacklist storage, Celery propagation)
- [x] JWT 라이브러리 선정 근거 (Go: golang-jwt/jwt v5, Python: PyJWT 2.8)
- [x] 7개 Risk (R-AUTH-001~007) detail + likelihood/impact/mitigation/residual
- [x] Cognitive bias check 4종 (anchoring/confirmation/sunk cost/overconfidence)
- [x] 5개 open question with sensible defaults
- [x] External references table (13 sources)
