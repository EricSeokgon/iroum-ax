# iroum-ax PoC Quickstart (5분)

한국 공공기관 경영평가 AI 자동화 플랫폼 — KEPCO E&C 안전보건 PoC 데모

> 모든 시연 데이터는 가상(합성) 데이터입니다. 실제 KEPCO E&C 자료와 무관합니다.

---

## 사전 요구

- Python 3.11+ (Poetry 또는 venv)
- Git

## 30초 데모 (mock 모드 — ML 모델 불필요)

```bash
git clone https://github.com/EricSeokgon/iroum-ax.git
cd iroum-ax
make setup        # Poetry 의존성 설치
make demo         # 5 Chapter 자동 실행 (mock, 30초 이내)
```

### make demo 실행 흐름

```
[demo-fixtures] 합성 픽스처 생성 (최초 1회)
  - tests/fixtures/synthetic/criteria/안전보건_평가편람.json
  - tests/fixtures/synthetic/guidelines/작성지침.txt
  - tests/fixtures/synthetic/reports/kepco-self-report-2026.json
  - tests/fixtures/synthetic/reports/other-A-grade-2026.json
  - tests/fixtures/synthetic/reports/other-B-grade-2026.json

[demo] 5 Chapter 순차 실행 (mock 모드)
```

---

## 실제 ML 모델 사용 (5-10분)

```bash
# 1. Docker 인프라 시작 (PostgreSQL + Redis)
make dev-up

# 2. ML 모델 사전 다운로드 (최초 1회, ~5GB)
#    Qwen 2.5 7B (텍스트 LLM)
huggingface-cli download Qwen/Qwen2.5-7B-Instruct
#    ko-sroberta-multitask (임베딩, ~600MB)
python -c "from sentence_transformers import SentenceTransformer; SentenceTransformer('jhgan/ko-sroberta-multitask')"

# 3. 실제 모델 데모 실행
make demo-real
```

---

## 출력 예시

```
============================================================
  iroum-ax PoC 데모 — KEPCO 안전보건 평가 자동화
  모드: MOCK
============================================================
  [주의] 모든 데이터는 시연용 합성 가상 데이터입니다.

============================================================
  Chapter 1: Document Ingestion (REQ-AX-001)
============================================================
  입력: KEPCO-2026-SH (한국전력기술(주) (가상 데이터))
  [OK] 문서 파싱 완료
  document_id: doc-demo-KEPCO-2026-SH
  텍스트 길이: 3,214자
  테이블 수: 1개
  처리 시간: 2.3초 (mock)

============================================================
  Chapter 2: Criterion Mapping (REQ-AX-002)
============================================================
  [OK] RAG 검색 완료 (p99 < 100ms)
    1. [SH-1-1] 신규 입사자 안전교육 이수율   relevance=0.94, max_points=15점
    2. [SH-1-2] 정기 안전교육 이수율          relevance=0.89, max_points=15점
    3. [SH-3-2] 자체 안전점검 시행            relevance=0.73, max_points=10점
  검색 시간: 47ms (mock)

============================================================
  Chapter 3: Grade Simulation (REQ-AX-003)
============================================================
  [OK] 등급 예측 완료 (mock)
  --- 등급 확률 분포 (3-way output) ---
  P(A): 0.420  ← A 등급 달성 가능성
  P(B): 0.450  ← 현재 등급 유지
  P(abstain): 0.130 ← 신뢰도 낮음
  예측 등급: abstain
  신뢰도: 낮음 — 사람 검수 권장
  확률 합 불변식: 0.42 + 0.45 + 0.13 = 1.00 (REQ-AX-003-E1 ✓)

============================================================
  Chapter 4: Report Draft Generation (REQ-AX-004)
============================================================
  [OK] 초안 생성 완료 (mock)
  --- 생성된 초안 (합니다체 검증 완료) ---

  당 기관은 안전보건 강화를 위하여 안전교육 이수율을 체계적으로
  관리하고 있습니다. [... 합니다체 초안 ...]

  합니다체 점수: 1.00 (100% 준수)
  생성 시간: 4.2초

============================================================
  Chapter 5: Gap Recommendation (REQ-AX-005)
============================================================
  [OK] Gap 분석 완료 — 상위 5개 추천 항목

  1위. 안전교육 이수율 100% 달성 [우선순위 높음]
       효과: +0.12 점수 개선, 실현가능성 92%
  2위. 정기안전 점검 월별 전환 [우선순위 높음]
       효과: +0.06 점수 개선, 실현가능성 78%
  [...]

  현재 P(A): 0.420
  추천 실행 후 P(A): 0.780 (예상)
  예상 등급: A (가능성 높음)

============================================================
  Audit Log 시뮬레이션 (REQ-UBI-003)
============================================================
  총 8개 감사 이벤트 기록 완료:
  [audit-001] UPLOAD         document/doc-demo-...
  [audit-002] WORKFLOW_CREATE workflow/wf-demo-001
  [audit-003] CRITERION_INDEX ...
  [...]

============================================================
  iroum-ax PoC 데모 — 최종 결과 요약
============================================================
  총 실행 시간: 0.4초 (mock 모드)

  [ 현재 상태 ]
    예측 등급: abstain (신뢰도 낮음 — 검토 권장)
    P(A) = 0.420 / P(B) = 0.450

  [ 개선 시뮬레이션 ]
    추천 항목 수: 5개
    추천 실행 후 P(A): 0.780
    예상 등급: A (가능성 높음)

  [ 핵심 가치 ]
    - 보고서 작성 초안 생성: ~4초 (vs 수작업 3-4시간/지표)
    - 평가기준 자동 매칭: 47ms (vs 수작업 수시간)
    - 모든 데이터 처리: 내부망 전용 (망분리 정합)

  데모 완료.
```

---

## 개별 타겟 실행

```bash
# 합성 픽스처만 재생성
make demo-fixtures-force

# 픽스처 삭제 후 재생성
make demo-clean && make demo-fixtures

# verbose 없이 간결한 출력
poetry run python pipelines/scripts/run_demo.py --mode=mock

# 픽스처 내용 확인
cat tests/fixtures/synthetic/criteria/안전보건_평가편람.json | python -m json.tool | head -30
cat tests/fixtures/synthetic/reports/kepco-self-report-2026.json | python -m json.tool | head -30
```

---

## 상세 문서

| 문서 | 위치 |
|------|------|
| 5 Chapter 데모 시나리오 | [.moai/demo/kepco-poc-walkthrough.md](../../.moai/demo/kepco-poc-walkthrough.md) |
| SPEC 요구사항 | [.moai/specs/SPEC-AX-001/spec.md](../../.moai/specs/SPEC-AX-001/spec.md) |
| 아키텍처 개요 | [.moai/project/codemaps/overview.md](../../.moai/project/codemaps/overview.md) |
| 파이프라인 코드맵 | [.moai/project/codemaps/pipelines.md](../../.moai/project/codemaps/pipelines.md) |

---

## 주의사항

- 모든 `tests/fixtures/synthetic/` 데이터는 시연 전용 합성(가상) 데이터입니다.
- 실제 KEPCO E&C 또는 타 기관의 자료가 아닙니다.
- 실제 ML 모델(Qwen 2.5, ko-sroberta)은 별도 다운로드가 필요합니다 (~5GB).
- mock 모드는 ML 모델 없이 30초 이내에 완전한 5 Chapter 흐름을 시연합니다.
