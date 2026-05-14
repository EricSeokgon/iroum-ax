# ADR 0007: Sandbox 환경 user_id 기본값: `cli-anonymous`

## Status

Accepted (2026-05-14)

## Context

**3 SPEC의 협력 모델**:
- **SPEC-AX-001** (Python 파이프라인): REQ-UBI-003에서 audit_logs.user_id 필드 정의
- **SPEC-AX-CTRL-001** (Go 제어 계층): audit_logs에 행 삽입, user_id propagate
- **SPEC-AX-AUTH-001** (OIDC 인증): 나중에 user_id를 JWT sub claim에서 추출

**문제**: PoC 단계에서 SPEC-AX-AUTH-001 미완성 → user_id를 어떻게 설정할 것인가?

선택지:
- **Path A (채택)**: `cli-anonymous` (표준 fallback)
- **Path B**: `anonymous` (모호함, SPEC 혼동 가능)
- **Path C**: `null` / empty string (NOT NULL 제약 위반)
- **Path D**: `system` (운영 자동화와 혼동)

## Decision

**AuthEnabled=false 시 audit_logs.user_id = 'cli-anonymous'** 기본값 설정.

근거:
- **명확성**: "사용자 인증 없는 CLI 환경" 의미 직관적
- **후향 호환성**: SPEC-AX-AUTH-001 도입 시 AuthEnabled=true 활성화 가능
  - 기존 cli-anonymous 감사 레코드는 그대로 유지
- **표준**: OAuth/OpenID 커뮤니티에서 anonymous는 관례
- **감사 추적**: 진정한 인증 사용자와 구분 가능

## Consequences

### 긍정적 영향

- **PoC 운영 단순화**: 인증 없이도 감사 로그 자동 기록
- **회귀 방지**: SPEC-AX-001 AC (REQ-UBI-003) 의존성 충족
- **명확한 의도**: 감사 리포트에서 "cli-anonymous" 행을 보면 PoC 단계 작동 명백

### 부정적 영향

- **실사용자 추적 불가**: PoC 로그에서 실명 또는 조직 정보 없음
  - Mitigation: 운영 단계 SPEC-AX-AUTH-001에서 Keycloak 인증 활성화 필수
- **규제 감시**: 감사원이 "모든 행동의 실사용자"를 요구할 경우
  - Mitigation: PoC 단계는 허용 (개발/검증용), 운영은 인증 필수

## Implementation

### Go Control Plane (apps/control-plane/internal/workflow/handlers.go)

```go
func (h *WorkflowHandler) resolveUserID(ctx context.Context) string {
    // Extract from JWT (future: SPEC-AX-AUTH-001)
    if claims := ctx.Value("user_claims"); claims != nil {
        return claims.(*JWTClaims).Subject
    }
    
    // Fallback for sandbox (current: PoC)
    return "cli-anonymous"
}

func (h *WorkflowHandler) CreateWorkflow(ctx context.Context, req *pb.CreateWorkflowRequest) (*pb.CreateWorkflowResponse, error) {
    userID := h.resolveUserID(ctx)
    
    // Insert workflow
    workflowID, err := h.store.CreateWorkflow(ctx, req.DocumentID, userID)
    if err != nil {
        return nil, err
    }
    
    // Dispatch to Celery
    // ...
    
    return &pb.CreateWorkflowResponse{WorkflowID: workflowID}, nil
}
```

### Python Pipelines (pipelines/config.py)

```python
# Celery task에서 user_id 추출 (envelope header에서)
@celery_app.task(bind=True)
def ingestion_worker(self, document_id: str, **kwargs):
    # Envelope headers에서 user_id 추출
    user_id = self.request.headers.get("user_id", "cli-anonymous")
    
    # Audit log 기록 (audit_logs 테이블)
    audit_log(
        user_id=user_id,
        action="DOCUMENT_INGESTED",
        resource_type="document",
        resource_id=document_id
    )
```

### Acceptance Criteria

| ID | 기준 |
|----|------|
| AC-UBI-001-A | AuthEnabled=false 시 모든 audit_logs.user_id = 'cli-anonymous' |
| AC-UBI-001-B | AuthEnabled=true 시 JWT sub claim으로 user_id 자동 추출 |
| AC-UBI-001-C | 기존 cli-anonymous 레코드는 AUTH-001 도입 후 그대로 유지 |
| AC-AUTH-UBI-001-C | SPEC-AX-AUTH-001 E2E 테스트에서 인증된 사용자 user_id 확인 |

## Backward Compatibility (운영 전환)

```
PoC 단계 (현재)
├─ SPEC-AX-001 GREEN
├─ SPEC-AX-CTRL-001 GREEN
├─ audit_logs.user_id = 'cli-anonymous' (모든 행)
└─ SPEC-AX-AUTH-001 = Plan (아직 구현 전)

SPEC-AX-AUTH-001 구현 (후속)
├─ AuthEnabled=false → 기존 'cli-anonymous' 경로 유지
├─ AuthEnabled=true → JWT sub 추출로 전환
├─ 기존 'cli-anonymous' 감사 레코드 마이그레이션 불필요 (이력 보존)
└─ Keycloak + JWKS 캐시 활성화

운영 단계
├─ AuthEnabled=true 강제 (KEPCO E&C 배포)
├─ 신규 감사 레코드만 실명 user_id 기록
└─ 'cli-anonymous'는 PoC 흔적 (역사적 기록)
```

## References

- SPEC-AX-001 research.md §3.4 (한국어 공문 스타일)
- SPEC-AX-001 spec.md REQ-UBI-001~003 (요구사항)
- SPEC-AX-CTRL-001 research.md §2.2 (Audit log enumeration)
- SPEC-AX-AUTH-001 spec.md REQ-AUTH-UBI-001 (인증 사용자 추적)
- `.moai/project/structure.md` §5 (audit_logs 테이블 스키마)

---

**작성자**: ircp  
**날짜**: 2026-05-14

**운영 팀 참고**: KEPCO E&C 배포 시점에 `AuthEnabled=true`를 필수로 설정하고, 모든 사용자 인증을 Keycloak OIDC로 강제합니다. Sandbox PoC에서는 `cli-anonymous` 기본값 유지로 개발 편의성 보장.
