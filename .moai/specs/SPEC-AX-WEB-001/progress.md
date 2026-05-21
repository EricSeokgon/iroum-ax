# SPEC-AX-WEB-001 — Progress Tracker

상태 진척 추적 파일. /moai run / /moai loop 각 iteration 종료 시 한 줄(또는 한 블록)을 append하여 stagnation gate(`spec-workflow.md` Re-planning Gate)가 비교할 수 있도록 한다.

스키마(iteration entry):
- timestamp (ISO-8601)
- phase: Plan | Run-A | Run-B | Run-C | Run-D | Run-E | Sync
- iteration: 1..N (해당 phase 안에서)
- AC_passed_count / AC_total
- new_errors_introduced (LSP errors delta from previous entry, optional)
- errors_fixed (delta)
- notes (한국어, 1-2줄)

---

## Sprint 계획 (Initial — 2026-05-21)

본 SPEC은 PoC 데모용 5-스크린 프런트엔드이며 백엔드 0-diff(HARD)이다. 신규 워크스페이스 `apps/web/`를 부트스트랩한 뒤 5+2 화면을 단계별 vertical slice로 구현한다. 각 Phase는 독립 PR 단위로 권장된다.

### Phase 분할 (Priority labels, no time estimates)

| Phase | 우선순위 | 범위 | 산출 AC | 의존 |
|-------|---------|------|---------|------|
| Phase A — 부트스트랩 + 인증 | High | 워크스페이스 scaffold(Next.js 14+ App Router + TS + Tailwind + shadcn/ui + ESLint/Prettier 통합), 루트 `package.json` workspace 갱신(OPEN #1 RESOLVED), 인증 베이스(REQ-WEB-001/001a/001b/008/008a + cross-cutting REQ-WEB-CROSS-001~005), RoleGate, SessionGuard, 401/403 handler | AC-001~004, AC-070~072 | OPEN #1/#2/#3 RESOLVED 필요 |
| Phase B — 증빙 | High | REQ-WEB-002 family (업로드 + 목록 + 상세 + viewer 비활성 + 100MB 가드) | AC-010~013 | Phase A |
| Phase C — 평가 항목 + 점수 | High | REQ-WEB-003/004 family (트리 + 항목 상세 + 점수 입력/수정 + viewer 비활성 + 백엔드 검증 표시) | AC-020~024 | Phase A |
| Phase D — 리포트 + 리뷰 | Medium | REQ-WEB-005 (범주 rollup·등급) + REQ-WEB-006 (Kanban 4-state 워크플로) | AC-030~031, AC-040~046 | Phase A, (옵션) Phase C |
| Phase E — 감사 로그 + 루브릭(admin) | Medium | REQ-WEB-007 (5-필터 + 페이지네이션 + admin only 라우트 가드) + REQ-WEB-009 (임계값 CRUD UI) | AC-050~051, AC-060~061 | Phase A |
| Phase F — Sync | Low | 시연 시나리오 1회 완주(AC-DEMO), Lighthouse 점검, README 작성, /moai sync | AC-DEMO | Phases A–E |

### 병렬화 권장

- Phase A 완료(인증 베이스 안정화) 후 Phase B/C/D/E는 화면 단위로 병렬 가능. file ownership이 컴포넌트 디렉터리(`components/evidences/`, `components/scores/` 등)로 자연 분리되므로 충돌 최소.
- 단, `lib/api/`·`lib/auth/`·`(dashboard)/layout.tsx`는 cross-cutting이므로 Phase A에서 안정화 후 후속 Phase는 read-mostly로 사용.

### 즉시 결정 필요(Phase A 시작 전, /moai plan annotation 단계 권장)

1. **OPEN #1** — 루트 `package.json` workspace 정합(`apps/console` 처리). 선택지 A(권장)/B/C 중 결정.
2. **OPEN #2** — Keycloak Realm 이름, web 전용 Public client ID, redirect URI, 시드 계정. `deployments/keycloak/realm-export.json` 점검 + 필요 시 사용자 확인.
3. **OPEN #3** — 토큰 저장 전략(HttpOnly 쿠키 BFF vs 클라이언트 메모리). 보안·구현 복잡도 트레이드오프, 사용자 결정 권장.

### Phase 중간 결정 가능

4. **OPEN #4** — 타입 동기화 전략(수동 vs OpenAPI 생성). Phase A 초입에 `apps/control-plane/`에 OpenAPI 산출물 존재 여부 확인 후 결정.
5. **OPEN #5/6/7/8** — 본 PoC 범위 외(§3 비목표) 후속 SPEC 후보.

---

## Iteration Log

(아직 entry 없음 — /moai plan 단계 완료 후 /moai run 시작 시 첫 entry append)

### Plan phase

- 2026-05-21: SPEC v0.1.0 draft 작성 완료. 3-file 구조 중 `spec.md`+`progress.md` 생성. `acceptance.md`는 spec.md §6 안에 통합되어 있어 별도 파일이 필요하면 후속 작업에서 분리 가능(또는 spec.md §6를 acceptance.md로 미러). Plan-auditor 통과 후 /moai run Phase A 진행 권장.

---

## Stagnation 감지 정책 (spec-workflow Re-planning Gate)

- 3 iteration 연속 AC_passed_count delta = 0 → stagnation flag
- new_errors_introduced > errors_fixed (해당 cycle) → drift warning
- 누적 scope drift > 30% (Phase별 planned files 대비 actual modified) → Re-planning Gate trigger

본 SPEC은 백엔드 0-diff(HARD)이므로 `apps/control-plane/**` 또는 `pipelines/**`가 modified 목록에 등장하면 즉시 drift 위반으로 처리하고 stop.
