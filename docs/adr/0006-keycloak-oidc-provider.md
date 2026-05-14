# ADR 0006: OIDC Provider: Keycloak 24.x LTS (전자정부 표준 후속 SPEC)

## Status

Accepted (2026-05-14)

## Context

**SPEC-AX-AUTH-001 단계: SSO/OIDC 도입 결정**

OIDC Provider 선택지:
- **Path A (채택)**: Keycloak 24.x LTS (자체 호스팅, K8s StatefulSet)
- **Path B**: 전자정부 표준 인증 (GPKI, 정부망 통합)
- **Path C**: Authentik (신생, 운영 사례 적음)
- **Path D**: Dex (federation 우수, standalone provider로는 기능 부족)

**규제 제약**:
- 망분리 정합: 외부 federation 불가
- 한국 운영 사례: ETRI, 금융사 다수 도입
- PoC vs 운영: PoC는 단일 인스턴스, 운영은 HA 필요

## Decision

**Keycloak 24.x LTS** 채택.

근거:

| 선정 기준 | Keycloak | 전자정부 표준 | Authentik | Dex |
|---------|---------|----------|-----------|-----|
| 자체 호스팅 | ✓ | △ | ✓ | ✓ |
| 망분리 정합 | ✓ | △ | ✓ | ✓ |
| OIDC 표준 | ✓ | △ | ✓ | ✓ |
| Helm Chart | ✓ (codecentric) | ✗ | ✓ | ✓ |
| 한국 사례 | 다수 | 공공 표준 | 적음 | 적음 |
| Local dev | ✓ (docker-compose) | ✗ | ✓ | ✓ |
| 통합 복잡도 | 중 | 높음 | 중 | 중 |

### 거부 사유

**전자정부 표준 인증**:
- 정부 인증서 발급, 정부망 통합 절차 필수
- Dev 환경 재현 불가능
- KEPCO E&C가 명시적 요구 전까지 deferral

**Authentik / Dex**:
- Keycloak에 비해 한국 운영 사례 적음
- 엔터프라이즈 기능 (HA, 감시) 부족

## Consequences

### 긍정적 영향

- **PoC 단계 빠른 배포**: docker-compose로 1줄 시작
- **확장성**: Realm 격리로 미래 멀티테넌트 가능
- **통합 생태계**: SAML, LDAP, OAuth federation 모두 Keycloak 단에서 지원
- **검증된 안정성**: Keycloak은 production-grade (Red Hat 후원)

### 부정적 영향

- **인프라 비용**: K8s StatefulSet + PostgreSQL backing store
- **운영 부담**: Keycloak 버전 업그레이드, 보안 패치 추적
- **Single Point of Failure**: PoC 단일 인스턴스 (R-AUTH-005 Medium risk)

### PoC → 운영 전환 로드맵

| 단계 | 시기 | 범위 |
|------|------|------|
| Phase 1 (현재) | 2026-05-14 | Keycloak 단일 인스턴스, Docker compose |
| Phase 2 (후속) | ANCHOR 고객 준비 | Keycloak HA (3-replica), Infinispan distributed cache |
| Phase 3 (옵션) | 전자정부 표준 요구 시 | SPEC-AX-AUTH-EGOV-001로 별도 SPEC |

## Implementation

### K8s Deployment (deployments/helm/keycloak/)

```yaml
apiVersion: v1
kind: StatefulSet
metadata:
  name: keycloak
spec:
  serviceName: keycloak
  replicas: 1  # PoC
  selector:
    matchLabels:
      app: keycloak
  template:
    metadata:
      labels:
        app: keycloak
    spec:
      containers:
      - name: keycloak
        image: quay.io/keycloak/keycloak:24.0.0
        env:
        - name: KEYCLOAK_ADMIN
          value: admin
        - name: KEYCLOAK_ADMIN_PASSWORD
          valueFrom:
            secretKeyRef:
              name: keycloak-secrets
              key: admin-password
        - name: KC_DB
          value: postgres
        - name: KC_DB_URL_HOST
          value: postgres.default.svc.cluster.local
        ports:
        - name: http
          containerPort: 8080
```

### OIDC Configuration (iroum-ax realm)

```json
{
  "realm": "iroum-ax",
  "enabled": true,
  "accessTokenLifespan": 3600,
  "refreshTokenMaxReuse": 0,
  "clients": [
    {
      "clientId": "iroum-ax-cli",
      "name": "iroum-ax CLI",
      "publicClient": true,
      "protocol": "openid-connect",
      "redirectUris": ["http://localhost:8081/callback"],
      "webOrigins": ["http://localhost:*"]
    }
  ]
}
```

### Acceptance Criteria

| ID | 기준 |
|----|------|
| AC-AUTH-001-1 | Keycloak realm 생성, OIDC discovery 활성 |
| AC-AUTH-001-2 | JWKS cache 1h hard TTL + stale-while-revalidate |
| AC-AUTH-001-3 | Go/Python JWT 검증 (allow-list: RS256, EdDSA, ES256) |
| AC-AUTH-001-4 | Token refresh + family invalidation (AC-AUTH-005-3) |
| AC-AUTH-001-5 | JWKS unavailable 시 cached keys 사용 (degraded mode) |

## References

- SPEC-AX-AUTH-001 research.md §2 (OIDC Provider 4-way comparison)
- SPEC-AX-AUTH-001 spec.md REQ-AUTH (인증 요구사항)
- Keycloak 공식 문서: https://www.keycloak.org/docs/latest/
- codecentric Helm Chart: https://github.com/codecentric/helm-charts

---

**작성자**: ircp  
**날짜**: 2026-05-14

**참고**: KEPCO E&C가 전자정부 표준 인증을 명시적으로 요구하는 경우, 별도 SPEC-AX-AUTH-EGOV-001을 작성하여 통합합니다. 현재는 Keycloak PoC로 진행합니다.
