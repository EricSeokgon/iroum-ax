# Sprint Contract — REQ-AX-002 Criterion Mapping (RAG Indexing)

- Sprint: Sprint 3
- SPEC: SPEC-AX-001 v0.1.2
- 작성일: 2026-05-14
- 작성자: manager-tdd (RED phase)
- Phase: RED (테스트 작성 단계)
- Harness: thorough

---

## 1. Priority Dimension

**Primary**: Functionality — 한국어 공공기관 도메인 RAG 파이프라인의 정확성
**Korean-language correctness is paramount**: 한자/한글 혼용 처리와 cold-start graceful 처리는 도메인 신뢰도에 직결

이유: Sprint 3는 6개 AC 중 4개가 edge case(AC-002-2, AC-002-4, AC-002-5, AC-002-6). 
한국어 공공 문서 도메인 특성상 한자/한글 혼용과 cold-start 시나리오는 
실제 KEPCO E&C PoC에서 첫 번째로 부딪히는 문제이므로 Completeness와 함께 우선순위를 둔다.

---

## 2. Acceptance Checklist

| AC | 설명 | 우선순위 | 테스트 마크 |
|----|------|---------|------------|
| AC-002-1 | 평가편람 indexing → top-3 검색, relevance > 0.7, 계층 구조 포함 | High | 기본 |
| AC-002-2 | 검색 결과 부족(0-2개) → `insufficient_context` 상태 명시 | High | 기본 |
| AC-002-3 | 항목→지표→배점 계층 구조 파싱 보존 + 한자/한글 정규화 | High | 기본 |
| AC-002-4 | HNSW 재구축 중 큐잉 또는 503 (stale 결과 금지) | Medium | integration |
| AC-002-5 | 한자 정규화 실패 시 raw fallback + confidence × 0.8 + no crash | High | 기본 |
| AC-002-6 | cold-start (빈 인덱스) → 명시적 응답, silent empty/500 금지 | High | 기본 |

---

## 3. Test Scenarios (API Contract Level)

### CriterionParser
- `parse(file_path)` → `list[Criterion]` (계층 파싱)
- 한자 포함 텍스트 → `normalization_warning` 메타데이터 첨부

### EmbeddingService
- `encode(text)` → `list[float]` 길이 768, norm > 0
- 한자 포함 텍스트도 encode 실패 없이 처리

### VectorStore
- `upsert(criteria)` → None (pgvector에 저장)
- `query(query_vec, top_k)` → `list[CriterionMatch]`
- 빈 인덱스 → `IndexNotBootstrappedError` 또는 명시적 반환

### Retriever
- `search(query, top_k=5)` → `list[CriterionMatch]`
- cold-start → `ColdStartResponse` 또는 동등 신호
- 한자 포함 쿼리 → confidence × 0.8 적용

---

## 4. Pass Conditions

- 모든 6개 AC 시나리오 자동화 테스트 통과 (GREEN phase 기준)
- 한자 정규화 실패 시 시스템 크래시 0건 (HTTP 500 발생 = 실패)
- cold-start silent empty 반환 0건
- `@pytest.mark.integration` 테스트는 testcontainers PostgreSQL+pgvector 사용
- 단위 테스트는 FakeVectorStore + mock_ko_sroberta 사용

---

## 5. Evaluator 4-Dimension Weight (Thorough Harness)

| 차원 | 가중치 | 이유 |
|------|--------|------|
| Functionality | 30% | RAG 파이프라인 정확성 |
| Completeness | 35% | 6개 AC 중 4개 edge case |
| Originality | 20% | 한국어 공공 도메인 RAG 특화 |
| Security | 15% | pgvector 쿼리 인젝션 방지 |

---

## 6. Sprint Contract Constraints (FROZEN)

- `pipelines/mapping/` 프로덕션 코드는 GREEN phase에서만 작성
- RED phase에서 `pkg/models/criterion.py` 스텁 확장만 허용 (로직 없음)
- testcontainers 사용 테스트는 반드시 `@pytest.mark.integration` 마킹
- ko-sroberta-multitask 실제 모델 로딩은 단위 테스트에서 금지 (mock 필수)
- 768차원 임베딩 벡터 크기 불변 (spec.md §6 D12 결정사항)

---

## 7. Sprint Contract Artifact

저장 위치: `.moai/sprints/SPEC-AX-001/sprint-REQ-AX-002.md`
다음 단계: GREEN phase 진입 (모든 RED 테스트 FAIL 확인 후)
