# ADR 0004: Go → Celery: Redis-direct Kombu v2 JSON envelope

## Status

Accepted (2026-05-14)

## Context

**아키텍처 계층**:
- **Go Control Plane** (apps/control-plane/): 워크플로우 오케스트레이션
- **Python Pipelines** (pipelines/): 5개 작업 (ingestion, mapping, scoring, generation, recommendation)

**문제**: Go가 Python Celery worker를 비동기로 dispatch해야 함.

선택지:
- **Path A (채택)**: Redis 직접 RPUSH + Kombu v2 JSON envelope
- **Path B**: asynq (Go-native 작업 큐, 하지만 Python 호환 불가)
- **Path C**: HTTP → FastAPI → Celery (한 계층 추가)
- **Path D**: RabbitMQ + AMQP (인프라 복잡)

## Decision

**Redis-direct Kombu v2 JSON envelope** 채택.

근거:
- **이미 존재하는 인프라**: SPEC-AX-001이 Celery worker(Redis broker) 사용
- **호환성**: Celery 5.3+ 표준 wire format
- **간결성**: Go에서 JSON 직렬화 후 Redis RPUSH
- **성능**: 지연 <50ms (직접 Redis 접근)
- **golden file 검증**: byte-for-byte 일치 테스트로 호환성 보증

## Consequences

### 긍정적 영향

- **최소 의존성**: Redis client 외 추가 라이브러리 불필요
- **빠른 dispatch**: gRPC → Redis RPUSH <10ms
- **Celery 표준 준수**: Kombu protocol v2 byte-exact 호환

### 부정적 영향

- **envelope 형식 관리**: Kombu 6.x 출시 시 breaking change 가능
  - Mitigation: version pinning (celery[redis] ^5.3.0)
- **golden file 유지**: testdata/celery_envelope_v2.json 수동 관리 필요
- **cross-language 복잡성**: Go/Python 양측 코드 변경 시 동기화 필수

## Implementation

### Go Side (apps/control-plane/internal/scheduler/celery_envelope.go)

```go
type CeleryMessage struct {
    Body       string            `json:"body"`                // base64(JSON([args, kwargs, embed]))
    Headers    map[string]interface{} `json:"headers"`        // task metadata
    Properties map[string]interface{} `json:"properties"`    // delivery metadata
}

func BuildIngestionTaskEnvelope(workflowID, documentID string) ([]byte, error) {
    // 1. Build positional args and kwargs
    args := []interface{}{documentID}
    kwargs := map[string]interface{}{"workflow_id": workflowID}
    embed := map[string]interface{}{
        "callbacks": nil,
        "errbacks": nil,
        "chain": nil,
        "chord": nil,
    }
    
    // 2. Serialize to JSON array
    bodyJSON := []interface{}{args, kwargs, embed}
    bodyBytes, _ := json.Marshal(bodyJSON)
    
    // 3. Base64 encode
    bodyB64 := base64.StdEncoding.EncodeToString(bodyBytes)
    
    // 4. Build envelope
    msg := CeleryMessage{
        Body: bodyB64,
        Headers: map[string]interface{}{
            "task": "pipelines.workers.ingestion_worker.run",
            "id": workflowID,
            "lang": "py",
            // ... other fields
        },
        Properties: map[string]interface{}{
            "correlation_id": workflowID,
            "delivery_mode": 2,
            "delivery_info": map[string]interface{}{
                "routing_key": "celery",
            },
        },
    }
    
    return json.Marshal(msg)
}
```

### Python Side Configuration

```python
# pipelines/config/settings.py
CELERY_TASK_SERIALIZER = "json"
CELERY_ACCEPT_CONTENT = ["json"]
CELERY_RESULT_SERIALIZER = "json"
```

### Golden File Test

```go
// apps/control-plane/internal/scheduler/celery_envelope_test.go
func TestEnvelopeV2Compatibility(t *testing.T) {
    envelope, _ := BuildIngestionTaskEnvelope(fixedTestUUID, fixedDocUUID)
    
    // Load golden file
    goldenBytes, _ := ioutil.ReadFile("testdata/celery_envelope_v2.json")
    
    // Compare byte-for-byte (stable JSON key order)
    if !bytes.Equal(envelope, goldenBytes) {
        t.Fatalf("envelope mismatch")
    }
}
```

## References

- SPEC-AX-CTRL-001 research.md §3 (Celery Protocol v2 상세)
- SPEC-AX-CTRL-001 spec.md REQ-CTRL-005 (Dispatch 요구사항)
- Celery 5.3 wire format: https://docs.celeryq.dev/en/v5.3.6/internals/protocol.html
- Kombu source: https://github.com/celery/kombu

---

**작성자**: ircp  
**날짜**: 2026-05-14
