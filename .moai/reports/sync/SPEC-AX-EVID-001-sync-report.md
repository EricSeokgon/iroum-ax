# SPEC-AX-EVID-001 Sync Report

**SPEC**: SPEC-AX-EVID-001 — 증빙 자료 수집/관리 (Evidence Management Walking Skeleton)  
**Sync Phase**: Phase 2 (Documentation Synchronization)  
**날짜**: 2026-05-18  
**평가**: evaluator-active PASS 0.930 | Coverage 91.4% | TDD RED-GREEN-REFACTOR  

---

## 1. 사전 승인 조건 이행 확인 (6-correction checklist)

Phase 2 Human Gate에서 확인된 6개 정확성 수정 사항을 모두 이행하였습니다.

| # | 수정 사항 | 이행 여부 | 근거 파일 |
|---|---------|---------|--------|
| 1 | `evidence_blobs` 테이블 없음 — `evidences` 단일 테이블만 존재 (`file_content BYTEA` 컬럼) | 이행 | `0002_evidence_tables.sql` 직접 확인 |
| 2 | 라우트 `POST /api/v1/evidences` 하나만 존재 (별도 `/versions` 라우트 없음) | 이행 | `evidence_handlers.go` `Routes()` 직접 확인 |
| 3 | `EvidenceBlobStore`/`dbBlobStore`는 논리적 위치만 반환 (`WriteTo`, `ReadFrom`, `DatabaseBlobStore` 타입 없음) | 이행 | `internal/storage/storage.go` 직접 확인 |
| 4 | 실제 REQ-EVID 제목 사용 (spec.md §3에서 추출) — REQ-EVID-001~004 + UBI-001~004 | 이행 | `spec.md` §3 직접 확인 |
| 5 | overview.md PostgreSQL 테이블 목록에 `evidences`만 기재 (`evidence_blobs` 미기재) | 이행 | overview.md 업데이트 확인 |
| 6 | 단일 핸들러 `handleCreateEvidence` (생성 + 버전 통합) — 두 핸들러 분리 없음 | 이행 | `evidence_handlers.go` 직접 확인 |

모든 6개 수정 사항 이행 완료.

---

## 2. 수정된 파일 목록

| 파일 | 수정 내용 요약 |
|------|-------------|
| `.moai/specs/SPEC-AX-EVID-001/spec.md` | `status: draft` → `status: completed`; `## Implementation Notes` 섹션 추가 (커밋, 브랜치, 날짜, TDD, evaluator-active PASS 0.930, coverage 91.4%, GAP-03/04, 기지 테스트 실패 공지) |
| `CHANGELOG.md` | `[Unreleased]` 아래 SPEC-AX-EVID-001 v0.1.0 `### Added`, `### Fixed`, `### Known` 3-파트 항목 추가 (Keep-a-Changelog 형식, 한국어) |
| `README.md` | 뱃지 `SPECs-7` → `SPECs-8`; 상태 줄 `+ 증빙 관리 완료` 추가; EVID-001 SPEC 블록 추가; 품질 섹션 `7개 SPEC` → `8개 SPEC` |
| `.moai/project/codemaps/go-control-plane.md` | `### 6. 증빙 관리` 섹션 신규 추가 (evidence.go, storage.go, recorder.go, clock.go, evidence_handlers.go, config.go 추가사항, errors.go 추가사항); 기존 §6 → §7 재번호 |
| `.moai/project/codemaps/req-traceability.md` | `## SPEC-AX-EVID-001` 섹션 추가 (REQ-EVID-001~004 + UBI-001~004 각각 구현 위치/테스트 매핑; GAP-01/03/04 해소 현황); 통합 요약 테이블 업데이트 (8→9행, 합계 업데이트, 날짜 갱신) |
| `.moai/project/codemaps/overview.md` | 3계층 다이어그램에 EVID-001 라우트 추가; PostgreSQL 목록에 `evidences` 추가; 모노레포 구조에 `storage/`, `errors/` 패키지 추가; SPEC 추적 테이블에 EVID-001 행 추가; PostgreSQL 테이블 목록 섹션 신규 추가 |

**변경하지 않은 파일** (scope discipline): `pipelines.md`, `data-flow.md`, `pkg.md`, `codemaps/README.md`, `plan.md`, `tasks.md`, `acceptance.md`, `contract.md`, `progress.md`, `strategy.md`, `postgres.go`

---

## 3. 문서별 변경 전/후 요약

### spec.md
- 변경 전: `status: draft`, Implementation Notes 없음
- 변경 후: `status: completed`, `## Implementation Notes` (7-항목 블록) 추가

### CHANGELOG.md
- 변경 전: `### Added — SPEC-AX-AUTH-003` 항목이 `[Unreleased]` 최상위
- 변경 후: EVID-001 3-파트 항목(`### Added`, `### Fixed`, `### Known`)이 AUTH-003 항목 앞에 삽입

### README.md
- 변경 전: 뱃지 SPECs-7, 상태 "ABAC 완료", EVID 블록 없음, "7개 SPEC"
- 변경 후: 뱃지 SPECs-8, 상태 "+ 증빙 관리 완료", EVID-001 블록 추가, "8개 SPEC"

### go-control-plane.md
- 변경 전: 5개 섹션 (§1 cmd/server, §2 internal/workflow, §3 internal/audit, §4 internal/store, §5 internal/scheduler, §6 설정 및 타입)
- 변경 후: `### 6. 증빙 관리` 신규 삽입 → 기존 §6 → §7로 재번호 (7개 섹션)

### req-traceability.md
- 변경 전: SPEC-AX-AUTH-003까지만, 통합 요약 53 AC / 380+ 테스트 / 43 모듈
- 변경 후: SPEC-AX-EVID-001 섹션 추가; 통합 요약 61+ AC / 480+ 테스트 / 49+ 모듈; 날짜 2026-05-18 갱신

### overview.md
- 변경 전: Go Control Plane v0.1.2, `evidences` 테이블 미기재, `storage/`/`errors/` 패키지 미기재, 4-SPEC 추적 테이블
- 변경 후: v0.1.3, `evidences` 다이어그램/테이블 추가, `storage/`/`errors/` 패키지 추가, 5-SPEC 추적 테이블, PostgreSQL 테이블 목록 섹션 신규 추가

---

## 4. 기지 이슈 (Sync 범위 외)

다음 사항은 SPEC-AX-EVID-001 구현 범위 이전부터 존재하는 기지 실패이므로 이 Sync에서 수정하지 않습니다.

- **Pre-existing test failure**: `apps/control-plane/cmd/server/e2e_test.go`에 SPEC-AX-EVID-001 구현 이전부터 존재하던 실패 테스트 1개. SPEC-AX-EVID-001 구현과 무관하며 후속 SPEC에서 처리 예정.

---

## 5. Phase 3 커밋 메시지 초안

```
docs(evid): SPEC-AX-EVID-001 v0.1.0 SYNC — TRUST 5 PASS 0.930 (증빙 관리 완료)

- spec.md: status draft → completed, Implementation Notes 추가
- CHANGELOG.md: [Unreleased] EVID-001 Added/Fixed/Known 항목 추가
- README.md: SPECs 7→8, 상태/뱃지/SPEC 블록 업데이트
- go-control-plane.md: §6 증빙 관리 섹션 추가 (storage, handlers, audit)
- req-traceability.md: SPEC-AX-EVID-001 REQ-EVID-001~004 + UBI-001~004 매핑 추가
- overview.md: evidences 테이블, storage/errors 패키지, SPEC 추적 업데이트

6-correction checklist: 모두 이행
- evidences 단일 테이블 (evidence_blobs 없음)
- POST /api/v1/evidences 단일 라우트
- dbBlobStore 논리 위치만 (WriteTo/ReadFrom 없음)
- 실제 REQ-EVID 제목 (spec.md §3 직접 추출)
- overview PostgreSQL 목록에 evidences만
- handleCreateEvidence 단일 핸들러
```

---

**보고서 생성**: manager-docs (claude-sonnet-4-6)  
**Ground-truth 확인 파일**: `0002_evidence_tables.sql`, `evidence_handlers.go`, `storage.go`, `spec.md`, `recorder.go`, `clock.go`, `errors.go`, `config.go`
