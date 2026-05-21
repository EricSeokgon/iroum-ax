# SPEC-AX-SCORE-API-001 Progress (TDD RED-GREEN-REFACTOR)

> sub-agent manager-tdd 순차 · thorough harness · 2026-05-19
> 각 태스크 RED→GREEN 증거 append-only. SCORE-001 dark-flow lesson: 증거 기반만, "should pass" 금지.

## T-001 [S0 BLOCKING GATE] — PASS (2026-05-19)

- S0-1 hard-verify: `grep 'func.*BeginScoreTx' pg_store.go store.go` → `pg_store.go:134:func (s *PgWorkflowStore) BeginScoreTx(ctx context.Context) (ScoreTx, error)` ✓. `store.go:255` = interface method spec `BeginScoreTx(ctx context.Context) (ScoreTx, error)` (no `func` keyword — 메서드 선언이므로 정상, sed 253-256 확인) ✓
- S0-2 hard-verify: `grep 'ErrGradeThresholdsUnavailable' errors.go` → `errors.go:69:var ErrGradeThresholdsUnavailable = errors.New(...)` ✓
- 회귀 baseline: `go build ./apps/control-plane/...` EXIT=0 · `go vet ./apps/control-plane/cmd/server/` EXIT=0 · `go test ./apps/control-plane/cmd/server/` → `ok ... 0.303s` (단위 GREEN, integration 태그 미사용)
- consumer-only Drift-Guard git blob 스냅샷 (HEAD d291efe): evidence_handlers.go=80082db, abac.go=eec8283, authz_middleware.go=bd13c51, chain.go=a01905d, rbac.go=31c670b, errors.go=5c59222, pg_store.go=17a1e8f, score.go=94ccf0f, store.go=b5f29e3
- 결론: consumer-only 전제 무손상 → T-002..T-015 진행 승인

---

## T-002~T-014 RED→GREEN 증거 (2026-05-19)

### RED (증거 기반 — 가짜 통과 아님)
`go test ./apps/control-plane/cmd/server/ -run 'TestScoreHandler|...'` 최초 실행:
```
undefined: ScoreHandler / NewScoreHandler / clampPagination / mapStoreErr / requireScoreWriteRole
FAIL ... [build failed]
```
→ acceptance.md 27 AC/16 edge를 단언하는 테스트가 구현 부재로 컴파일 실패 (진성 RED-first).

RED 중 발견된 잘못된 테스트 가정 1건 수정: `TestScoreHandler_UBI_NoSelfAudit_NoForge`가
"GET은 BeginScoreTx 미진입" 단언 → store.go:276 `GetScoreByID`는 ScoreTx 메서드라
읽기도 TX 경유가 인터페이스 설계상 필연. UBI-002-2 실제 계약("API 자체 audit 0건")으로
교정: 읽기 TX는 commit 없이 rollback, InsertScore/Update/Supersede 미호출 단언(더 강한 검증).

### GREEN (증거)
- build: `go build ./apps/control-plane/...` EXIT=0
- vet: `go vet ./apps/control-plane/cmd/server/` EXIT=0
- score 단위: `ok ... 0.037s` — 39 PASS / 0 FAIL (goleak inline VerifyNone 포함)
- 회귀: `go test ./apps/control-plane/cmd/server/` → `ok ... 0.310s` (evidence/workflow REST baseline 무손상)
- gofmt -l: 빈 출력 (양 파일 포맷 OK)
- goleak fault-inject: TXRollback_OnDownstreamFailure/BeginFailure, Update/Supersede CommitFailure_Rollback,
  CreateScore_StoreError 전부 PASS — TX rollback 검증 + goroutine 누출 0

goleak 해결 노트: `defer goleak.VerifyNone` + `t.Parallel()` 병행은 형제 러너 goroutine을
false-positive로 포착(evidence_handlers_test.go 선례 동형). goleak 테스트에서 t.Parallel() 제거로 해결.

### 27 AC / 16 edge 커버리지 (genuine RED-first 단언)
- UBI-001 (외부의존0/store위임): T-014 UBI_NoSelfAudit + 정적 grep (http.Client/SDK/os/exec/audit SQL 0건)
- UBI-002 (mutation 1 audit / 자체 audit 0): T-008 CreateScore_201(BeginTx→Insert→Commit) + UBI_NoSelfAudit (recorder 미주입, GET TX rollback)
- UBI-003 (cli-anonymous/auth-disabled): T-013 AuthDisabled_Passthrough (context 부재 투과)
- UBI-004 (권한·불변): ViewerWriteDeny_403 / UpdateScore_409_Immutable / Supersede_409(DRAFT+SUPERSEDED 2분기)
- 001-1~7 (조회): GetScore_200/404/400 · ListScores_FilterAndSlice/Empty/BlankEvalItem · Rollup_NumericPrecision · Grade_200/404/400 · clampPagination(6분기) · PaginationClamp
- 002-1~5 (변경): CreateScore_201/400(5종) · UpdateScore_200/409/400 · Supersede_201/409/400(3종)
- 003-1~3 (ABAC): RequireScoreWriteRole(5분기) · ViewerWriteDeny_403 · ViewerRead_200 · AdminBypass_200 · AuthDisabled_Passthrough
- 004-1~2 + BOUNDARY-1: MapStoreErr(7센티넬+unknown+wrapped) · TXRollback fault-inject · BOUNDARY-1 git 해시 0-diff
- §7 edge 16: #1 ViewerWriteDeny / #2 GetScore_404 / #3 Update_409 / #4·#14 Supersede_409 2분기 / #5·#7 Create_400 /
  #6 GetScore_400_MalformedUUID / #8·#9·#10 clampPagination / #11 AuthDisabled / #12 ListScores_Empty /
  #13 Grade_404 / #15 RoutesRegistersSevenPatterns(최장일치) / #16 BOUNDARY-1 0-diff

## T-015 BOUNDARY-1 + 품질 (2026-05-19) — PASS

- BOUNDARY-1 consumer-only 0-diff: frozen 9파일 git blob 해시 = T-001 baseline 완전 동일
  (evidence_handlers.go 80082db / abac.go eec8283 / authz_middleware.go bd13c51 / chain.go a01905d /
   rbac.go 31c670b / errors.go 5c59222 / pg_store.go 17a1e8f / score.go 94ccf0f / store.go b5f29e3)
- 변경 파일 = 정확히: score_handlers.go [NEW] · score_handlers_test.go [NEW] · server.go [MODIFY +7줄: scoreH 필드+NewScoreHandler+innerMux.Handle 2줄] · SPEC docs
- server.go diff: 마운트 전용 — ABAC 와이어링(auth.ABACMiddleware) 무변경 (diff hunk가 그 라인 이전에서 종료)
- 신규 마이그레이션 0, 자체 audit SQL/Recorder 0, 신규 외부 의존 0 (production import: net/http,encoding/json,errors,strconv,strings,uuid,zap,internal/{auth,errors,store})
- 커버리지: score_handlers.go 250/261 statements → **95.79%** (≥85% 충족)
- golangci-lint default: 0 issues · gosec: 0 issues (--fix로 fieldalignment 정규화)
- goleak: fault-inject 포함 전 핸들러 누출 0
- @MX: ANCHOR 2(ScoreHandler/Routes)+REASON · WARN 4(mapStoreErr/3 TX handlers)+REASON · NOTE 1 · RED TODO→GREEN 해소 · ko 준수 · per-file 한도(ANCHOR≤3, WARN≤5) 내

전 15 태스크 RED→GREEN→REFACTOR 완결. SCORE-001 dark-flow lesson 적용: 모든 단언 증거 기반.
