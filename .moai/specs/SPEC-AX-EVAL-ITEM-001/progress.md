# SPEC-AX-EVAL-ITEM-001 진행 기록 (Progress)

> TDD RED-GREEN-REFACTOR, thorough harness. Sprint별 AC 완료 수 + 에러 델타 추적 (정체 감지용).
> 회귀 baseline (T-001 [EXISTING], 2026-05-18): store integration suite GREEN (184.5s, 0 failures), audit suite GREEN (0 failures). 이것이 TH-11 무회귀 기준선.

## Sprint 진행 로그

| Sprint/Task | 단계 | AC 누적 완료 | LSP 에러 (build/vet) | golangci-lint | 비고 |
|-------------|------|--------------|----------------------|---------------|------|
| 기준선 | baseline | 0 / 22 | 0 | (미측정) | store+audit integration GREEN 캡처 완료 |
| T-001 | GREEN | 0 / 22 (인프라) | 0 | (미측정) | 0003 마이그레이션 + 7 migration tests PASS (DC-001 전부, E-15). schema.sql에 evaluation_items 추가 (testcontainers 부트스트랩, initial.sql 미수정). |
| T-002 | GREEN | 0 / 22 (골격) | 0 | (미측정) | EvalItemStore/EvalItemTx 인터페이스 + Action 상수 + EvalItemAuditNamespace 고정 UUID. 2 tests PASS (DC-002.3/2.4). additive-only 128+/0- 확인. |
| T-003 | GREEN | 6 / 22 | 0 | (미측정) | 루트 생성+metadata semantic RT+센티널 4 tests PASS (DC-003 전부, E-11/E-12). InsertEvalItem에 T-004 parent/입력 검증 동시 구현. |
| T-004 | GREEN | 9 / 22 | 0 | (미측정) | 자식 생성+parent orphan 거부+입력검증(blank/65자/dup PK)+BeginEvalItemTx pool재사용 5 tests PASS (DC-004 전부, E-01/E-18). pg_store.go additive-only, postgres.go/initial.sql 불변 (TH-13). |
| T-005 | GREEN | 11 / 22 | 0 | (미측정) | 계층 자기참조 조회 6 tests PASS (DC-005 전부, E-17/GAP-02). EXPLAIN Index Scan 검증 (test-data 선택도 수정 — 인덱스 정의는 T-001 검증됨). |
| T-006 | GREEN | 13 / 22 | 0 | (미측정) | FK RESTRICT + hierarchy_code UNIQUE 2 tests PASS (DC-006 전부, E-02/E-03). |
| T-007 | GREEN | 16 / 22 | 0 | (미측정) | AUD-1 deterministic UUIDv5 recorder 6 tests PASS (DC-007 전부, E-07/08/09, DC-010.1/10.2). recorder.go additive-only, ANCHOR 28/37/246/272 intact. SEC-07 nolint 적용. |
| T-008 | GREEN | 17 / 22 | 0 | (미측정) | 양방향 rollback (create+update) 2 tests PASS (DC-008 전부, E-10/E-16). goleak 0. |
| T-009 | GREEN | 21 / 22 | 0 | (미측정) | 계층 불변성+라이프사이클 6 tests PASS (DC-009 전부). **GAP-01 [BLOCKER] LeafNodeParentIDChangeSucceeds PASS** — guard 과일반화 없음 검증. |
| T-010 | GREEN | 22 / 22 | 0 | EXIT 0 (zero issues) | UBI+경계 3 tests PASS (DC-010.3~10.9, E-13/E-14). Quality Gate: build/vet 0 errors, golangci-lint 0 issues, eval-item 신규코드 커버리지 86.9% (≥85% — eval_item.go 87.0%, recorder eval funcs 86.4%), goleak 0 (rollback tests), 신규 외부dep 0 (go.mod 불변). |

## 최종 Quality Gate (T-010 / Sprint 5)

| 게이트 | 결과 | 근거 |
|--------|------|------|
| `go build ./...` | PASS (0 errors) | 전체 빌드 성공 |
| `go vet ./...` (unit+integration) | PASS (0 errors) | LSP 게이트 클린 |
| `go test ./...` (unit) | PASS (0 failures) | 11 패키지 ok |
| eval-item integration tests | PASS (store 300s, audit clean) | 25 eval_item + 7 migration + 2 rollback + 6 recorder tests GREEN |
| 회귀 baseline (Workflow/Evidence/PgStore integration) | PASS (382.9s, 0 failures) | TH-11/DC-010.13 무회귀 |
| `golangci-lint run ./...` | EXIT 0 (zero issues) | TH-Linter/DC-010.11, SEC-07 nolint:gosec 적용 |
| 신규 eval-item 코드 커버리지 | 86.9% (≥85%) | DC-010.10/TH-coverage |
| goleak | 0 leak | DC-008.4/8.5 rollback tests, audit TestMain VerifyTestMain |
| initial.sql / postgres.go / go.mod / evidences | 불변 (git diff 0) | TH initial.sql/postgres.go/no-new-dep/evidences immutability |
| brownfield @MX:ANCHOR (store.go:18/34/59/71, recorder.go:28/37/246/272) | intact (additive-only) | TH brownfield @MX, DC-002.5/DC-007.11 |
| @MX:TODO 잔존 | 0 | TH MX:TODO resolved |
| 22 AC | 22/22 GREEN | AC→DC 매트릭스 전부 충족 |

## Targeted Fix Cycle (iteration 1/3, 2026-05-18 — pre-sync polish)

> evaluator-active Phase 2.8a PASS (96/95/82/97) 이후 NON-blocking polish 2건. 동작/SQL/로직 무변경 순수 정리.

| 결함 | 심각도 | 조치 | 검증 |
|------|--------|------|------|
| M1 (Readable/Craft, TH-06) `UpdateEvalItem` ≤50줄/≤10복잡도 초과 | MEDIUM | `eval_item.go:206` orchestrator 94→**45줄** 분해. 헬퍼 추출: `validateStatusTransition` (eval_item.go:254, 상태 화이트리스트), `checkHierarchyMutationGuard` (eval_item.go:270, successor 가드 — 검증순서 [HARD] 주석으로 PgEvalItemTx @MX:WARN 계약 명시), `buildEvalItemUpdateSet` (eval_item.go:293, 하드코딩 컬럼 SET fragment+args 반환). SEC-02 불변식 유지(컬럼명 하드코딩, 값만 $N). $N 번호 동치성 보존(`len(args)+1` ≡ 기존 `argN`). | build/vet 0, golangci-lint EXIT 0, eval_item.go 커버리지 86.2% (statement-weighted, ≥85%). GAP-01 `LeafNodeParentIDChangeSucceeds`/`ChildBearing{ParentID,Level}ChangeRejected`/`StatusLifecycleTransitions`/`InvalidStatusRejected`/`AuditFailRollback_{Create,Update}Path` 전부 PASS — 동작 무변경. integration store 194~404s GREEN. |
| M2 (stale @MX:TODO) `audit.go:86` | LOW | 만료 `@MX:TODO - Sprint 1에서 PostgreSQL audit_logs INSERT 구현` (CTRL-001/EVID-001에서 이미 구현·운영 중) → `@MX:NOTE`로 정정 (store-layer Tx PgWorkflowTx/PgEvidenceTx/PgEvalItemTx.InsertAuditLog 현황 기술). 스코프 규율: 이 1개 stale 태그만 수정, 기존 @MX:ANCHOR 무변경. | `grep @MX:TODO audit.go` = 0건. recorder.go/store.go/pg_store.go @MX:ANCHOR 삭제·이동 0 (TH-11 무회귀). |

**불변식 무회귀 (fix-cycle 후 재확인)**: initial.sql / postgres.go / 0002_evidence_tables.sql / go.mod / go.sum / evidences-table-in-schema.sql git diff = 0 (TH-07/08/09/10). brownfield @MX:ANCHOR additive-only (삭제 0). 신규 외부 dependency 0. evaluation_items.id VARCHAR(64)·AUD-1·EvalItemAuditNamespace 고정 UUID 상수 무변경.
