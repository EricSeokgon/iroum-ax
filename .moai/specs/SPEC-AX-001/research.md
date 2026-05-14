# SPEC-AX-001 Research — 안전보건 PoC Walking Skeleton

> 본 문서는 SPEC-AX-001 작성을 위한 사전 조사 결과를 정리합니다.
> 대부분의 컨텍스트는 `.moai/project/` 하위 문서에 이미 존재하므로, 본 research는 해당 자료를 인용하고 SPEC 결정에 필요한 추가 분석만 추가합니다.

- 작성일: 2026-05-14
- 작성자: ircp
- 대상 SPEC: SPEC-AX-001
- 프로젝트 단계: Greenfield (Phase 1 — Anchor 고객 확보)

---

## 1. 사전 조사 출처 (Primary Sources)

| 영역 | 출처 | 핵심 인용 라인 |
|------|------|----------------|
| 제품 비전·MVP 5기능 | `.moai/project/product.md` §5.1-5.5 | L137-223 (5개 MVP 기능 정의) |
| Anchor 고객·PoC 항목 | `.moai/project/product.md` §3.2 | L62-78 (KEPCO E&C, 안전보건, 고객 제공 자료 4종) |
| Phase 4+ 금융권 후보 (제외 대상) | `.moai/project/product.md` §8.1 | L319-325 (금융권은 본 SPEC 범위 외) |
| Walking Skeleton 슬라이스 정의 | `.moai/project/interview.md` §10 | L148-156 (안전보건 PoC Walking Skeleton 6개 조건) |
| 모노레포 디렉토리 트리 | `.moai/project/structure.md` §2 | L34-254 (apps/control-plane Go, pipelines/* Python, apps/console TS, schemas/proto) |
| 모듈 책임 경계 | `.moai/project/structure.md` §3.1-3.2 | L262-287 (ingestion/mapping/scoring/generation/recommendation) |
| 데이터 플로우 | `.moai/project/structure.md` §4 | L292-307 (Console → Control Plane → Workflow → Celery → Pipelines → DB) |
| DB 스키마 개요 | `.moai/project/structure.md` §5 | L313-365 (documents/criteria/reports/workflows/simulations) |
| VLM 선택 (Qwen2-VL 7B/72B) | `.moai/project/tech.md` §2.1-2.2 | L57-76 (한국어 OCR, vLLM 호환, Apache 2.0) |
| LLM 선택 (EXAONE 3.5 / Qwen 2.5 fallback) | `.moai/project/tech.md` §3.1-3.3 | L82-104 (EXAONE 3.5 7B/32B, 대체 전략) |
| RAG 스택 (pgvector + ko-sroberta-multitask) | `.moai/project/tech.md` §4 | L110-145 (HNSW 인덱스, 청킹 500-1000 토큰) |
| HWP 파싱 (hwp-converter + OCR fallback) | `.moai/project/tech.md` §5 | L151-191 (한컴 OLE 구조, OCR 2차 대체) |
| 망분리·PII·감사로그 | `.moai/project/tech.md` §9.1-9.4 | L332-418 (NetworkPolicy, PII regex, audit_logs 테이블) |
| 성능 목표 | `.moai/project/tech.md` §11.1-11.2 | L474-487 (OCR <2s/page, RAG <100ms, draft <5s, prediction <1s) |
| 위험 관리 | `.moai/project/tech.md` §13.1-13.2 | L531-544 (VLM/EXAONE/pgvector/vLLM 위험 매트릭스) |
| 품질 모드 | `.moai/config/sections/quality.yaml` | development_mode: tdd, test_coverage_target: 85, session_effort_default: xhigh |

본 SPEC은 위 출처를 단일 진실 공급원(SSOT)으로 삼고, 충돌 시 product.md > structure.md > tech.md 순으로 우선합니다.

---

## 2. 아키텍처 의존성 그래프 (structure.md §2 인용 기반)

```
[Console (TypeScript, apps/console/)]
   └── (본 SPEC 범위 외 — UI는 의도적으로 deferral)
        │
[Control Plane (Go, apps/control-plane/)]
   ├── main.go
   ├── cmd/server/ (gRPC :50051 + REST :8080)
   ├── internal/workflow/ ← 상태머신 (Workflow 테이블 관리)
   ├── internal/scheduler/ ← Celery 태스크 dispatch
   └── internal/store/ ← PostgreSQL + pgvector 클라이언트
        │ gRPC (schemas/proto/workflow.proto)
        ▼
[Python Pipelines (pipelines/)]
   ├── ingestion/ (REQ-AX-001)
   │    ├── hwp_parser.py        ← hwp-converter (1차)
   │    ├── pdf_parser.py        ← pypdf + OCR (1차)
   │    ├── vlm_processor.py     ← Qwen2-VL 7B via vLLM (1차 + HWP/PDF 폴백)
   │    └── table_extractor.py   ← VLM 후처리 (테이블 셀 정렬)
   │
   ├── mapping/ (REQ-AX-002)
   │    ├── criterion_parser.py  ← 평가편람 항목→지표→배점 계층 파싱
   │    ├── embedding_service.py ← ko-sroberta-multitask (170M)
   │    ├── vector_store.py      ← pgvector HNSW 인덱스
   │    └── retriever.py         ← top-k 검색 (k=5, 재순위 3)
   │
   ├── scoring/ (REQ-AX-003)
   │    ├── benchmark_learner.py ← A/B 등급 보고서 특징 추출
   │    ├── grade_predictor.py   ← 확률 분포 산출 (B vs A 2-class)
   │    └── scenario_simulator.py← B→A 점수 시나리오
   │
   ├── generation/ (REQ-AX-004)
   │    ├── llm_client.py        ← EXAONE 3.5 7B (1차) / Qwen 2.5 7B (대체)
   │    ├── prompt_builder.py    ← 평가지표별 프롬프트 템플릿
   │    └── style_applier.py     ← 한국어 공문 합니다체 스타일 검증·재생성
   │
   ├── recommendation/ (REQ-AX-005)
   │    ├── gap_analyzer.py      ← 현재 vs 목표 등급 콘텐츠 차이
   │    ├── content_suggester.py ← 벤치마크 기반 콘텐츠 항목 제안
   │    └── prioritizer.py       ← 실현 가능성 우선순위
   │
   ├── workers/
   │    ├── ingestion_worker.py
   │    ├── generation_worker.py
   │    └── simulation_worker.py
   │
   └── main.py (FastAPI)
        │
        ▼
[데이터 계층]
   ├── PostgreSQL + pgvector (documents/criteria/reports/workflows/simulations)
   ├── Redis (Celery broker, 결과 캐시)
   └── vLLM (Qwen2-VL 7B, EXAONE 3.5 7B — 단일 GPU sandbox)
```

의존성 방향 (DAG, structure.md §10.1 인용):

```
Schemas (Proto) → Control Plane (Go) → Pipelines (Python) → vLLM / pgvector / Redis
```

본 SPEC은 위 DAG에서 단일 슬라이스를 통과시키는 것을 목표로 하며, Console 계층은 의도적으로 제외합니다.

---

## 3. 한국 공공부문 규제 제약 조건 (tech.md §9 인용)

### 3.1 망분리 정합 (tech.md §9.1)

- 본 PoC는 외부 LLM API 호출 금지 (OpenAI/Claude/Gemini 등 사용 불가)
- 모든 추론은 자체 호스팅 vLLM (Qwen2-VL, EXAONE 3.5/Qwen 2.5)으로 수행
- K8s NetworkPolicy로 egress 차단 (sandbox 단일 노드에서도 동일 정책 적용 권장)
- Walking Skeleton에서는 단일 노드 sandbox 사용 (full K8s 배포는 본 SPEC 범위 외)

### 3.2 PII 마스킹 (tech.md §9.2)

- 본 SPEC 단계에서는 기본 regex 마스킹만 적용 (전화번호, 한글 인명 2-4자)
- 고급 마스킹 규칙 (직책·내부 식별자·민감 실적)은 후속 SPEC으로 이관
- 마스킹은 `pkg/models/document.py` 로딩 시점 적용 (Inbound 위치 확정)

### 3.3 감사 로그 (tech.md §9.3)

- 본 SPEC은 audit_logs 테이블 스키마 정의 및 최소 이벤트 기록 (문서 업로드, 워크플로 생성, 초안 생성)만 포함
- 감사 로그 UI는 본 SPEC 범위 외 (SPEC-AX-002 이후 후보)

### 3.4 기획재정부 경영평가 편람 구조 (product.md §5.2)

- 계층: 항목(예: 안전보건) → 지표(예: 안전교육 이수율) → 배점(예: 5점)
- criterion 레코드는 부모-자식 관계 보존 (`parent_criterion_id` 필드 권장)
- 안전보건 항목은 PoC에서 1개 지표만 처리 (Walking Skeleton 원칙)

### 3.5 한국어 공문 스타일 특성

- 합니다체 기본 (예: "수행하였습니다", "달성하였습니다")
- 한자/한글 혼용 가능 (예: "안전(安全)")
- 부서·직책 표기는 공식 명칭 사용
- 능동/수동 일관성: 보고서는 능동형 우선
- 검증은 style_applier.py에서 수행 (REQ-AX-004 unwanted clause)

---

## 4. 위험 등록부 (Risk Register)

위험 ID와 완화 전략은 plan.md `## Risk Analysis` 섹션에서 상세화됩니다. 본 research에서는 식별된 위험만 정리합니다.

| ID | 위험 | 출처 | 영향 | 1차 완화 |
|----|------|------|------|----------|
| R-AX-001 | HWP OLE 구조 손상 시 파싱 실패 | tech.md §5.3 | Document Ingestion 차단 | VLM OCR 폴백 (REQ-AX-001 unwanted) |
| R-AX-002 | Qwen2-VL 7B의 테이블 셀 인식 정확도 부족 | tech.md §11.1 | OCR 95% 미달 위험 | 7B에서 정확도 측정 → 미달 시 72B 승격 (production gate) |
| R-AX-003 | RAG 콜드스타트 (안전보건 학습 코퍼스 제한) | product.md §10.1 | 검색 top-3 relevance 0.8 미달 | 청크 오버샘플링 + 수동 relevance 검증 |
| R-AX-004 | EXAONE 3.5 액세스 불확실 | tech.md §3.3 | 초안 생성 불가 | Qwen 2.5 7B fallback (자동 분기) |
| R-AX-005 | K8s GPU 자원 부족 | tech.md §6.1 | 추론 5-10배 지연 | CPU 추론 경로 + sandbox 단일 GPU 권장 |
| R-AX-006 | 공문 스타일 (합니다체) 위반 생성 | product.md §5.4 | 사용자 만족도 저하 | style_applier 재생성 루프 (최대 3회) |
| R-AX-007 | pgvector HNSW 인덱스 튜닝 부재 | tech.md §11.1 | RAG p99 100ms 초과 | 초기 코퍼스 인덱싱 후 ef_construction/m 파라미터 측정·고정 |
| R-AX-008 | 한자/한글 혼용 임베딩 품질 저하 | research.md §3.5 | criterion 매핑 정확도 저하 | 정규화 전처리 (한자→한글 변환 옵션) + ko-sroberta 베이스라인 비교 |

---

## 5. 참조 구현 (Reference Implementations)

본 SPEC 구현 시 참고할 외부 자료:

| 영역 | 참조 | 용도 |
|------|------|------|
| Qwen2-VL 모델 카드 | https://huggingface.co/Qwen/Qwen2-VL-7B-Instruct | VLM 프롬프트·입력 형식 |
| vLLM 공식 문서 | https://docs.vllm.ai/ | 배치·페이지드 어텐션 설정 |
| vLLM 성능 튜닝 | https://docs.vllm.ai/en/latest/performance_tuning.html | OCR <2s/page 달성 |
| pgvector HNSW 가이드 | https://github.com/pgvector/pgvector#query-performance | RAG p99 <100ms 인덱싱 |
| ko-sroberta-multitask | https://huggingface.co/jhgan/ko-sroberta-multitask | 한국어 임베딩 (170M) |
| hwp-converter | https://github.com/mete0r/pyhwp 또는 동등 패키지 README | HWP OLE 파싱 패턴 |
| EXAONE 3.5 (참고) | LG AI Research (TBD — 액세스 미확정 시 Qwen 2.5로 대체) | 한국어 공문 생성 |
| Qwen 2.5 (대체) | https://huggingface.co/Qwen/Qwen2.5-7B-Instruct | EXAONE 미가용 시 fallback |
| FastAPI | https://fastapi.tiangolo.com/ | Pipelines 엔트리포인트 |

---

## 6. 기존 코드 분석 (Greenfield Note)

본 프로젝트는 Greenfield 단계로 `apps/`, `pipelines/`, `pkg/`, `schemas/` 디렉토리에 구현 코드가 존재하지 않습니다 (structure.md L5 "상태: 계획 단계").

따라서 본 SPEC은 다음 원칙을 따릅니다:

- Delta marker (`[EXISTING]`, `[MODIFY]`, `[REMOVE]`) 미사용
- 모든 파일은 신규 작성 (`[NEW]` 가정)
- structure.md §2 디렉토리 트리에 정의된 경로만 인용 (새 경로 추가 시 structure.md 갱신 필요 — 본 SPEC은 기존 트리 내에서만 작업)
- 신규 파일 경로가 structure.md §2와 일치하지 않으면 SPEC 작성 중단 후 structure.md 우선 갱신 필요 (현재 모든 후보 경로는 §2 내 위치 확인 완료)

---

## 7. SPEC 범위 결정 — Walking Skeleton 경계

### 7.1 포함 범위 (interview.md §10 인용)

- 평가편람 1개 항목 (안전보건) 파싱 → criterion 레코드 생성
- KEPCO E&C 실적보고서 1개 HWP 파싱 → document 레코드 생성
- 평가기준 ↔ 자사 보고서 매핑 (RAG top-k 검색)
- 보고서 초안 1개 섹션 생성 (1개 평가지표 대상)
- 등급 시뮬레이션 1회 (A 등급 vs B 등급 2-class 분류, C/D는 제외)
- Gap recommendation 3-5개 항목 (B→A 상향 시나리오)
- CLI/API 진입점 (FastAPI + control-plane REST/gRPC)

### 7.2 의도적 제외 (spec.md Exclusions 섹션에서 상세화)

- Console UI (apps/console/) — 전체 미구현
- 다중 문서 배치 처리 — 1 문서만
- 500개 평가항목 — 안전보건 1개만
- 인접 도메인 (ESG/감사/면허/공문) — Phase 3 후보
- 금융권 도메인 — Phase 4+ 후보 (product.md §8.1)
- 멀티테넌트 분리 — 단일 테넌트
- 프로덕션 K8s 배포 — sandbox 단일 노드
- 감사 로그 UI — 스키마와 최소 기록만
- 모델 파인튜닝 — 베이스 모델 (Qwen2-VL 7B, EXAONE 3.5 7B) 그대로 사용
- PII 고급 마스킹 규칙 — 기본 regex만
- 평가 등급 C/D 분류 — A/B 2-class만 (벤치마크 데이터 활용 우선순위)
- SSO/JWT 인증 — TBD (structure.md §3.2)

---

## 8. EARS 결정 — 5개 요구사항 모듈

5개 MVP 기능 (product.md §5.1-5.5)을 1:1 EARS 모듈로 매핑합니다. 통합·분할 후보 검토 결과:

- **분할 검토**: REQ-AX-001 Document Ingestion을 HWP 파서 / VLM OCR / 테이블 추출로 분할 후보 → 거부. 이유: 폴백 체인이 자연스러운 하나의 흐름이며, 3개로 분할 시 7개 모듈로 늘어 5개 한도 초과
- **통합 검토**: REQ-AX-004(생성) + REQ-AX-005(추천)을 통합 후보 → 거부. 이유: 생성은 LLM 호출 흐름, 추천은 벤치마크 데이터 기반 분석 흐름으로 입력·출력 데이터·실행 시점이 분리

최종 모듈:

| ID | 책임 | 1차 의존성 |
|----|------|------------|
| REQ-AX-001 | Document Ingestion | hwp-converter, Qwen2-VL 7B via vLLM |
| REQ-AX-002 | Criterion Mapping (RAG) | ko-sroberta-multitask, pgvector |
| REQ-AX-003 | Grade Simulation | A/B 벤치마크 데이터셋, scikit-learn or simple feature extractor |
| REQ-AX-004 | Report Draft Generation | EXAONE 3.5 7B (1차) / Qwen 2.5 7B (대체) via vLLM |
| REQ-AX-005 | Gap Recommendation | REQ-AX-003 출력 + 벤치마크 콘텐츠 인덱스 |

---

## 9. 본 research가 SPEC에 미치는 영향 요약

- spec.md는 위 9개 출처를 EARS 5개 모듈로 변환
- plan.md는 §4 위험 등록부 + §5 참조 구현을 1:1 매핑
- acceptance.md는 §3 한국어 공공부문 규제 제약을 edge case로 변환
- spec-compact.md는 본 research를 제외하고 EARS + Acceptance + Affected files + Exclusions만 추출

---

**References**:
- product.md §3.2, §5.1-5.5, §8.1, §10.1
- structure.md §2, §3.1-3.2, §4, §5, §10.1
- tech.md §2.1-2.2, §3.1-3.3, §4, §5, §6.1, §9.1-9.4, §11.1-11.2, §13.1-13.2
- interview.md §0, §3, §10
- quality.yaml (development_mode: tdd, test_coverage_target: 85)
