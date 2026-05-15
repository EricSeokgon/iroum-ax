# SPEC-AX-OBS-001 — 인수 기준 (acceptance.md)

Given/When/Then 형식. REQ당 최소 2 AC, REQ-OBS-UBI-001 4개 sub-clause는 각각 dedicated AC 보유 (lessons #2). Total AC: 18.

---

## REQ-OBS-UBI-001 (Ubiquitous — 4 sub-clause dedicated AC)

### AC-OBS-UBI-001-a (망분리 — pull only, push gateway 금지)

- Given: 전체 control-plane 바이너리가 정상 부팅된 상태
- When: 소스 전체에서 push gateway 관련 심볼(`push.New`, `prometheus/client_golang/prometheus/push`)을 grep하고, `cfg.OTelEnabled=false`(default)로 서버를 띄운 뒤 외부 outbound 연결을 관측
- Then: push gateway import/호출이 0건이며, default 상태에서 OTLP/외부 메트릭 backend로의 outbound 연결이 0건이다 (`/metrics`는 inbound pull only)

### AC-OBS-UBI-001-b (무영향 계측 — overhead < 1ms p99)

- Given: `InstrumentHTTP`로 래핑된 no-op 핸들러
- When: `go test -bench=BenchmarkInstrumentHTTP`를 실행하여 래핑 유/무 latency를 측정
- Then: 계측 추가 overhead p99 < 1ms이며, instrumentation hot path에 blocking I/O(DB/network) 호출이 없다(코드 검토 + 벤치 단언). PgPool gauge 갱신은 요청 경로가 아닌 별도 goroutine에서만 발생

### AC-OBS-UBI-001-c (RBAC 보호 — read:metrics 필수)

- Given: `cfg.AuthEnabled=true`, OBS metrics permission registry 활성
- When: `metrics.IsMetricsAuthorized([]auth.Role{auth.RoleAdmin})`와 `metrics.IsMetricsAuthorized([]auth.Role{auth.RoleViewer})`를 각각 호출
- Then: admin은 true, viewer는 false를 반환하며, `auth.permissionMatrix`(rbac.go)는 변경되지 않았다(`git diff rbac.go` 빈 결과 단언 — frozen 보존)

### AC-OBS-UBI-001-d (PII 미노출 — label cardinality bound)

- Given: 7개 collector가 등록되고 다양한 요청(상이한 workflow ID 포함 path)이 처리된 상태
- When: registry를 gather하여 모든 metric family의 label value 집합을 수집
- Then: path label은 정규화 라우트 패턴(`/api/v1/workflows/{id}` 형태, 실제 UUID 미포함)만 존재하고, 사용자 식별자/문서 내용/raw body가 label value로 나타나지 않으며, 라벨 조합 카디널리티가 정적 상한 이내이다

---

## REQ-OBS-001 — Metrics Registry & Collectors

### AC-OBS-001-1 (7 collector 등록)

- Given: 깨끗한 프로세스 상태
- When: `metrics` 패키지 싱글톤을 초기화하고 registry를 `Gather()`
- Then: 정확히 7개 metric family(`iroum_ax_http_request_duration_seconds`, `iroum_ax_grpc_request_duration_seconds`, `iroum_ax_workflow_state_transitions_total`, `iroum_ax_celery_dispatch_total`, `iroum_ax_pg_pool_connections`, `iroum_ax_auth_rejections_total`, `iroum_ax_authz_forbidden_total`)가 canonical 이름으로 노출된다

### AC-OBS-001-2 (중복 등록 멱등 — REQ-OBS-001-E1)

- Given: 싱글톤이 이미 1회 초기화된 상태
- When: 초기화 함수를 추가로 2회 더 호출
- Then: 동일 registry 인스턴스가 반환되고 `prometheus.AlreadyRegisteredError` panic이 발생하지 않으며 collector가 재등록되지 않는다

### AC-OBS-001-3 (default registry 미사용 — REQ-OBS-001-U1)

- Given: 본 SPEC collector가 모두 싱글톤 registry에 등록된 상태
- When: `prometheus.DefaultGatherer.Gather()`를 호출
- Then: 본 SPEC의 7개 collector가 default registry에 존재하지 않는다(전역 상태 회피 단언)

---

## REQ-OBS-002 — /metrics Endpoint + RBAC

### AC-OBS-002-1 (admin → 200 exposition — REQ-OBS-002-E1/S1)

- Given: `cfg.AuthEnabled=true`, 유효한 `iroum-ax:admin` 토큰
- When: `GET /metrics` 요청
- Then: HTTP 200, `Content-Type: text/plain; version=0.0.4`, body에 `iroum_ax_` prefix 메트릭 exposition이 포함된다

### AC-OBS-002-2 (authz_mapping 등록 — REQ-OBS-002-E2)

- Given: AUTH-002 `authz_mapping.go`에 `/metrics` 행이 추가된 상태
- When: `auth.LookupRESTPermission("GET", "/metrics")` 호출
- Then: `(perm="read:metrics", bypass=false, found=true)`를 반환한다(default-deny 503 미적용 — 매핑 발견)

### AC-OBS-002-3 (unauthenticated → 401 — REQ-OBS-002-U1)

- Given: `cfg.AuthEnabled=true`, Authorization 헤더 없음
- When: `GET /metrics` 요청
- Then: HTTP 401, body `{"error":{"code":"UNAUTHENTICATED",...}}`, 메트릭 exposition 미노출

### AC-OBS-002-4 (viewer → 403 — REQ-OBS-002-U2)

- Given: `cfg.AuthEnabled=true`, 유효한 `iroum-ax:viewer` 토큰
- When: `GET /metrics` 요청
- Then: HTTP 403, body `{"error":{"code":"PERMISSION_DENIED","details":{"required":"read:metrics"}}}`, `iroum_ax_authz_forbidden_total{role="viewer",method="GET /metrics"}`가 1 증가, exposition 미노출

### AC-OBS-002-5 (AuthEnabled=false backward-compat)

- Given: `cfg.AuthEnabled=false`
- When: 인증 헤더 없이 `GET /metrics` 요청
- Then: HTTP 200 + exposition (SERVER-001 backward-compat 정합 — 인증 없이 통과)

---

## REQ-OBS-003 — HTTP/gRPC Instrumentation

### AC-OBS-003-1 (REST 최외곽 계측 — REQ-OBS-003-E1)

- Given: `InstrumentHTTP`가 `auth.BuildRESTChain` 외곽에 래핑된 서버
- When: 유효 토큰으로 `GET /api/v1/workflows` 1건, 그리고 토큰 없이 같은 경로 1건(401) 요청
- Then: `iroum_ax_http_request_duration_seconds{method="GET",path="/api/v1/workflows",status="200"}` count=1, `{...,status="401"}` count=1 (인증 실패도 최외곽이므로 계측됨)

### AC-OBS-003-2 (gRPC 최외곽 계측 — REQ-OBS-003-E2)

- Given: metrics interceptor가 auth chain 앞에 composed된 gRPC 서버
- When: 유효 토큰으로 `CreateWorkflow` 1건, 권한 부족 토큰으로 1건(PermissionDenied) 호출
- Then: `iroum_ax_grpc_request_duration_seconds{method=".../CreateWorkflow",code="OK"}` count=1, `{...,code="PermissionDenied"}` count=1

### AC-OBS-003-3 (probe 경로 제외 — REQ-OBS-003-S1)

- Given: 정상 부팅 서버
- When: `GET /health`, `GET /ready`, `GET /metrics`를 각각 호출 후 registry gather
- Then: 세 경로에 대한 `iroum_ax_http_request_duration_seconds` 시계열이 생성되지 않는다(self-scrape recursion + SLA 오염 방지)

### AC-OBS-003-4 (panic 시 관측 + re-panic — REQ-OBS-003-U1)

- Given: 의도적으로 panic하는 테스트 핸들러를 `InstrumentHTTP`로 래핑
- When: 해당 핸들러로 요청
- Then: `iroum_ax_http_request_duration_seconds{...,status="500"}` count=1이 기록되고, panic이 상위로 re-raise되어 기존 recovery 미들웨어 동작이 변경되지 않는다

---

## REQ-OBS-004 — OpenTelemetry Skeleton

### AC-OBS-004-1 (noop default — REQ-OBS-004-S1)

- Given: `cfg.OTelEnabled=false` (default)
- When: `metrics.InitTracing(cfg)` 호출
- Then: noop TracerProvider + non-nil idempotent shutdown 함수가 반환되고, span 생성 시 외부 outbound 연결이 0건이다

### AC-OBS-004-2 (OTLP opt-in — REQ-OBS-004-O1/E1)

- Given: `cfg.OTelEnabled=true`, `cfg.OTLPEndpoint`가 유효한 내부 stub collector 주소
- When: `InitTracing` 후 request 처리 (workflow create + celery dispatch 포함)
- Then: request span 하위에 `workflow.create`와 `celery.dispatch` child span이 생성되고 W3C traceparent가 propagate된다

### AC-OBS-004-3 (init 실패 non-fatal — REQ-OBS-004-U1)

- Given: `cfg.OTelEnabled=true`, `cfg.OTLPEndpoint`가 도달 불가 주소
- When: `InitTracing` 호출 후 서버 부팅 진행
- Then: 구조화 warning 로그 후 noop provider로 fallback하고, 서버 startup이 abort되지 않으며 `/metrics`·요청 처리가 정상 동작한다

---

## Edge Cases / 회귀

### AC-OBS-EDGE-1 (label cardinality 상한 강제)

- Given: 정의되지 않은(매핑 없는) 임의 path로 다수 요청
- When: registry gather
- Then: path label이 정규화 패턴 집합 또는 단일 `unmatched` 버킷으로 수렴하여 시계열이 무한 증가하지 않는다

### AC-OBS-EDGE-2 (shutdown race — goroutine leak 없음, lessons #12)

- Given: PgPool 갱신 goroutine + tracer가 동작 중인 부팅된 서버
- When: SIGTERM 전송 후 `go.uber.org/goleak`로 goroutine 누수 검사
- Then: graceful shutdown 후 PgPool goroutine과 tracer가 정리되어 누수 goroutine이 0이다(SERVER-001 sync.Once 정합)

### AC-OBS-EDGE-3 (SERVER-001/AUTH-002 regression)

- Given: 본 SPEC 변경 적용
- When: `go test -race ./...` 전체 실행
- Then: SERVER-001 `TestServer*` + AUTH-002 RBAC 테스트 + 기존 ~445 테스트가 모두 통과한다(unchanged)

---

## Definition of Done

- [ ] AC-OBS-* 18건 전부 GREEN
- [ ] 테스트 커버리지 ≥ 85% (`quality.yaml`)
- [ ] `go test -race ./...` 전체 통과 (SERVER-001/AUTH-002 regression 포함)
- [ ] `BenchmarkInstrumentHTTP` overhead p99 < 1ms, `BenchmarkMetricsScrape` p99 < 50ms
- [ ] push gateway 코드 부재(grep 0건) + OTLP default noop 단언
- [ ] `rbac.go` permissionMatrix 미변경(frozen 보존, git diff 빈 결과)
- [ ] AUTH-002 §13 + SERVER-001 §4·§5 cross-SPEC unblock fact 확인
- [ ] MX 태그 4종 추가 (registry ANCHOR / InstrumentHTTP NOTE / goroutine WARN+REASON / wiring NOTE), `code_comments: ko`
- [ ] EARS 5 패턴 모두 사용 검증 (Ubiquitous/Event/State/Optional/Unwanted)
