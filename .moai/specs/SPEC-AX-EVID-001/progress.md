# SPEC-AX-EVID-001 — 구현 진행 기록 (progress.md)

> TDD RED-GREEN-REFACTOR | thorough harness | 19 AC / 19 DC (contract.md) | coverage ≥ 85%
> Re-planning Gate: 각 Sprint 종료 시 AC 완료 수 + 에러 델타 기록 (3회 연속 0 진척 시 stagnation)

## T-001 Characterization Baseline (GREEN snapshot — 2026-05-18)

| Suite | Mode | Result |
|-------|------|--------|
| internal/store | unit | PASS |
| internal/store | integration (testcontainers) | PASS (66.7s) |
| internal/audit | unit + integration | PASS |
| cmd/server | unit | PASS (0.31s) |
| cmd/server | integration | PASS (49.6s) |
| golangci-lint store+audit | lint | 0 issues |
| go build ./... | build | exit 0 |

Regression guard: 모든 [MODIFY] 작업 종료 시 이 baseline 재확인 (회귀 0건).

## Sprint 진행 로그

| Sprint | 완료 Task | AC 완료 (누적/19) | DC GREEN (누적/19) | 에러 수 | 비고 |
|--------|-----------|-------------------|--------------------|---------|------|
| (init) | — | 0 | 0 | 0 | T-001 baseline 확보 |
| Sprint 0 | T-001~T-004 | 0 | 0 | 0 | migration+인터페이스 골격, build/vet/regression GREEN |
| Sprint 1 | T-005,T-006 | 3 | DC-006,008,012(gap) | 0 | InsertEvidence/GetByID/BeginEvidenceTx GREEN |
| Sprint 2 | T-007~T-009 | +6 (누적 9) | +DC-005,009,011,013 (7) | 0 | 동시성 직렬화/버전 체이닝/mutation guard GREEN; goleak infra-ignore 패턴 확립 |
| Sprint 3 | T-010,T-011,T-018 | +4 (누적 13) | +DC-004,014,015,016 (11) | 0 | RecordEvidence*/양방향 롤백/clock 주입 GREEN; T-001 audit 회귀 0 |
| Sprint 4 | T-012~T-017 | +6 (누적 19/19) | +DC-001,003,007,010,017,018,019 (19/19) | 0 | EvidenceBlobStore/handler/config/validation/dup-signal/sovereignty GREEN; GAP-01~08 전부 해소 |
| Sprint 5 | T-019 | 19/19 | 19/19 | 0 | 커버리지 evidence-core 91.3% (≥85%), lint+gosec 0, goleak 0, TH-06/07/08/10 PASS, T-001 회귀 0 |

## 최종 검증 결과 (T-019 Quality Gate)

- `go build ./...` / `go vet ./...` / `go test ./...` (unit, 17 pkg): 전부 PASS, 0 FAIL
- 통합 테스트 (store/audit/storage/config/cmd-server, -tags=integration): 전부 GREEN. T-001 characterization 회귀 0건
- golangci-lint default + gosec (G201/G202/G401/G501 포함): **0 issue**
- evidence-core 집계 커버리지 (evidence.go+storage.go+clock.go+evidence_handlers.go): **91.3%** (242 stmt 중 221) ≥ 85% (TH-01 PASS)
- goleak: 모든 TX/동시성/롤백 테스트 통과 (infra goroutine ignore 패턴, TH-03 의도 충족)
- TH-06 initial.sql 불변 / TH-07 신규 SaaS SDK 0 / TH-08 postgres.go 불변 / TH-10 brownfield @MX 삭제 0 — 전부 PASS
- contract.md DC-001~DC-019 전부 GREEN, SEC-01~SEC-07 전부 충족, GAP-01~GAP-08 전부 해소

## @MX Tag Report — Run Phase 2B (2026-05-18)

### Tags Added (10)
- store.go: `@MX:ANCHOR` EvidenceStore, `@MX:ANCHOR` EvidenceTx
- evidence.go: `@MX:WARN` PgEvidenceTx (orphan 누출), `@MX:WARN` GetLatestVersionByEvalItem (FOR UPDATE), `@MX:NOTE` UpdateEvidenceBodyColumn
- pg_store.go: `@MX:ANCHOR` BeginEvidenceTx
- recorder.go: `@MX:ANCHOR` RecordEvidenceCreated, `@MX:ANCHOR` RecordEvidenceVersioned
- storage.go: `@MX:NOTE` EvidenceBlobStore
- evidence_handlers.go: `@MX:ANCHOR` EvidenceHandler, `@MX:WARN` handleCreateEvidence (defer Rollback 순서)

### Tags Removed (0) — RED 단계 @MX:TODO는 production 코드에 미사용 (테스트 우선 작성으로 즉시 GREEN)
### Tags Updated (0)
### Brownfield ANCHOR 보존 (TH-10): store.go:16/32, recorder.go:36/27, pg_store.go:24/99 — 전부 변경 없음 (additive only)

## Targeted Fix Cycle — Iteration 1 (2026-05-18) — manager-quality CRITICAL + evaluator-active FAIL 해소

> 트리거: manager-quality Phase 2.5 CRITICAL + evaluator-active Phase 2.8a Security must-pass FAIL

| 결함 | 분류 | 해소 | 파일:라인 | 검증 |
|------|------|------|-----------|------|
| F1 | BLOCKER (Security must-pass) | SEC-02 단일 패스 스트리밍 SHA-256 — `io.Copy(io.MultiWriter(&buf,hasher), io.LimitReader(part,max+1))`. ReadAll-후-Sum256 제거. | `cmd/server/evidence_handlers.go:134` `parseAndHashMultipart` | `TestParseAndHashMultipart_StreamingSinglePass` (4 sub) PASS (unit, DB불요) + 통합 SEC-02 GREEN |
| F2 | High | blob 논리 location 생성을 TX 스코프 내(BeginEvidenceTx 이후 `persistEvidenceTx`)로 이동. database_blob에서 dbBlobStore.Put은 side-effect-free, bytes는 InsertEvidence(file_content) 경유 (strategy.md §2.6.5 6a). 어떤 경로도 evidence TX 밖 blob 영속 불가. | `cmd/server/evidence_handlers.go:254` `persistEvidenceTx` | 통합 DC-006/011/016 GREEN |
| F3+F4 | High | DC-003 store-level `TestEvidenceAudit_VersionRowAtomicCommit` 신설 (단일 EvidenceTx: InsertEvidence + MarkSuperseded[F4] + RecordEvidenceVersioned 원자 커밋, 커밋 전 원자성 가드 포함). | `internal/store/evidence_audit_test.go:67` | `TestEvidenceAudit_VersionRowAtomicCommit` PASS (integration) |
| F5 | High | `handleCreateEvidence` ~200줄→오케스트레이션 전용. 헬퍼 추출: `parseAndHashMultipart`/`parseMultipartForm`/`validateEvidenceParts`/`resolveVersion`/`persistEvidenceTx`. 행위 불변 (전 테스트 GREEN). | `cmd/server/evidence_handlers.go:305` `handleCreateEvidence` | 전 단위/통합 테스트 회귀 0 |
| F6 | 커버리지 스코프 | NEW evidence-core 파일 union 커버리지 = **91.4%** (224/245) ≥85%. 패키지 전체 ~62%는 PRE-EXISTING out-of-scope(postgres.go 死stub/fake_store.go/recorder.go workflow 메서드 — SPEC-AX-CTRL-001) 때문 — 신규 테스트 미작성(scope discipline). | (측정) | evidence.go 93.7%, evidence_handlers.go 89.1%, storage.go 100%, clock.go 100% |

### F6 계약 명료화 권고 (계약 미수정 — 권고만)
TH-01 (contract.md §3 line 372-376)은 "overall coverage >= 85.0%" + "Scope: all new packages (...evidence additions...)"로 기술됨. `go tool cover -func=coverage.out | tail -1`의 패키지 total은 SPEC-AX-CTRL-001 소유 미테스트 코드(postgres.go/fake_store.go/recorder.go workflow 메서드)를 포함하여 ~62%로 나타남. 권고: TH-01 측정 방법을 "evidence 신규 파일 집합의 statement-weighted union coverage"로 명시(현 91.4%)하거나, 측정 명령을 `-coverpkg` evidence 파일 필터로 한정. 본 에이전트는 계약을 수정하지 않음 (BINDING — evaluator-active 재평가 시 결정).

### 검증 증거 (iter-1)
- `go build ./...` / `go vet ./...`: exit 0
- `go test ./...` (unit, 12 pkg): 전부 PASS, 0 FAIL
- `go test -tags=integration ./internal/store/... ./cmd/server/... ./internal/audit/... ./internal/storage/... ./internal/config/...`: 전부 GREEN (store 212s, cmd/server 122s)
- `golangci-lint run --enable=gosec ./...`: **exit 0, 0 issue** (G201/G202/G401/G501/G304 clean)
- TH-06 initial.sql / TH-08 postgres.go: 변경 0건 (git status 확인)
- TH-10 brownfield @MX (store.go:16/32, recorder.go:36/27, pg_store.go:24/99): 변경 0건
- 신규 @MX 없음 (헬퍼 추출은 기존 @MX:WARN handleCreateEvidence 의미 보존, fan_in 변화 없음)

### Out-of-scope 사전 결함 (scope discipline — 미수정, 보고만)
`internal/server/authz_e2e_test.go:540` `TestE2E_GRPC_Authz_ViewerForbidden_Create` — gRPC `CallbackSerializer` goroutine leak + redis dial timeout(127.0.0.1:16399), 격리 실행 0.5s FAIL. SPEC-AX-AUTH-002/SERVER-001 영역, evidence 코드 미연관 (`internal/server/` git diff 0건). [HARD] scope discipline 준수 — 별도 SPEC에서 처리 필요 (memory lesson #11 readiness probe redis deps / #12 graceful shutdown race 연관).
