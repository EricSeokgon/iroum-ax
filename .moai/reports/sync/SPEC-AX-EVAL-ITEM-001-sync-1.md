# Sync Report — SPEC-AX-EVAL-ITEM-001

**날짜**: 2026-05-19  
**단계**: Phase 3 (Sync)  
**SPEC 버전**: v0.1.3  
**구현 커밋**: `fe53095` (2026-05-18)

---

## Anti-Phantom 검증 결과

| 검증 항목 | 결과 |
|-----------|------|
| 단일 `evaluation_items` 테이블 (추가 테이블 없음) | PASS — `0003_eval_item_tables.sql` 확인, 다른 테이블 없음 |
| HTTP 엔드포인트 없음 (store/audit 계층 전용) | PASS — 모든 문서에서 "HTTP 엔드포인트 없음" 명시, cmd/server 무변경 |
| `resource_id` = UUIDv5 (원시 계층 코드 아님) | PASS — `uuid.NewSHA1(EvalItemAuditNamespace, []byte(hierarchyCode))` 기록 |
| `EvalItemAuditNamespace` 정확한 UUID literal | PASS — `a7f3c2e1-9b4d-5e6f-8a0b-1c2d3e4f5a6b` (audit.go:82 확인) |
| 정확한 REQ-EVALITEM 제목 (spec.md §3) | PASS — UBI-001~004 + 001~004 모두 spec.md에서 직접 확인 |
| 에러 센티널 5종 정확한 이름 (errors.go) | PASS — `ErrEvalItemNotFound`, `ErrEvalItemInvalidInput`, `ErrEvalItemParentNotFound`, `ErrEvalItemHierarchyImmutable`, `ErrEvalItemInvalidStatus` |
| `BeginEvalItemTx` pool 재사용 패턴 (pg_store.go) | PASS — `PgWorkflowStore.pool` 단일 pgx 풀 재사용 확인 |

**Anti-Phantom 7/7 PASS**

---

## 수정 파일 요약

| 파일 | 변경 요약 |
|------|-----------|
| `.moai/specs/SPEC-AX-EVAL-ITEM-001/spec.md` | `status: draft` → `status: completed`; `## Implementation Notes` 섹션 추가 |
| `CHANGELOG.md` | `[Unreleased]` 최상단에 SPEC-AX-EVAL-ITEM-001 Added/Deferred 블록 추가; 기존 `TestE2E_GRPC_Authz_ViewerForbidden_Create` Known 항목 보존 (중복 없음) |
| `README.md` | SPEC 배지 8→9 GREEN; 상태 라인에 EVAL-ITEM-001 추가; 평가항목 taxonomy Walking Skeleton 섹션 추가; SPEC 카운트 업데이트 |
| `.moai/project/codemaps/go-control-plane.md` | 제목에 SPEC-AX-EVAL-ITEM-001 추가; 새 §7 평가항목 taxonomy 섹션 삽입; 기존 §7(설정)→§8, §7(Protobuf)→§9 번호 조정 |
| `.moai/project/codemaps/req-traceability.md` | SPEC-AX-EVAL-ITEM-001 섹션 추가 (REQ-EVALITEM-UBI-001~004 + REQ-EVALITEM-001~004, 실제 테스트 파일명 검증); 통합 요약 테이블 업데이트 |
| `.moai/project/codemaps/overview.md` | `evaluation_items` 테이블 추가 (PostgreSQL 목록 + 3계층 아키텍처 다이어그램); SPEC 추적 테이블 업데이트; store/ 주석 업데이트; Go Control Plane SPEC-version 라인 업데이트 |

---

## 사전 확인 — 기지 실패 항목

아래 항목은 본 SPEC 이전부터 존재하는 기지 실패이며 이번 Sync 범위에 포함되지 않습니다.

- `TestE2E_GRPC_Authz_ViewerForbidden_Create`: SPEC-AX-AUTH-002/SERVER-001 범위의 pre-existing 실패, SPEC-AX-EVAL-ITEM-001 범위 외

---

## 불일치 사항

해결되지 않은 불일치 없음.

---

## Phase 3 커밋 메시지 초안

```
docs(eval-item): SPEC-AX-EVAL-ITEM-001 v0.1.3 SYNC — TRUST 5 PASS 0.905 (평가항목 taxonomy 완료)

- spec.md: status draft→completed, Implementation Notes 추가
- CHANGELOG.md: SPEC-AX-EVAL-ITEM-001 Added/Deferred 블록 (Keep-a-Changelog)
- README.md: SPEC 배지 8→9 GREEN, 평가항목 taxonomy Walking Skeleton 섹션
- go-control-plane.md: §7 평가항목 taxonomy 섹션 추가, 섹션 번호 조정
- req-traceability.md: SPEC-AX-EVAL-ITEM-001 REQ 매핑 (8 REQ, 22 AC, 5 test files)
- overview.md: evaluation_items 테이블 추가, SPEC 추적 업데이트

Anti-Phantom 7/7 PASS:
- 단일 evaluation_items 테이블 (추가 테이블 없음)
- HTTP 엔드포인트 없음 (store/audit 계층 전용)
- resource_id = uuid.NewSHA1(EvalItemAuditNamespace, hierarchyCode) UUIDv5
- EvalItemAuditNamespace = a7f3c2e1-9b4d-5e6f-8a0b-1c2d3e4f5a6b (불변 상수)
```
