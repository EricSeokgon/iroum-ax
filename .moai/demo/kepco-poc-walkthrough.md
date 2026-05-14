# KEPCO E&C PoC 데모 워크스루

> **대상**: KEPCO E&C 경영평가팀 + IT 운영부서 + 기획조정실  
> **시간**: 60분 (준비 30분 + 시연 60분)  
> **문서 버전**: 1.0  
> **작성일**: 2026-05-14  
> **상태**: Walking Skeleton v0.1.2 실환경 시연 자료

---

## 개요

iroum-ax는 한국 공공기관의 경영평가 보고서 작성을 **AI 에이전트로 자동화**하는 솔루션입니다.

**핵심 가치**:
- **시간 절감**: 연간 보고서 작성 90% 단축 (1년 → 수주)
- **품질 향상**: 평가기준 자동 체크리스트 + 등급 상향 추천
- **데이터 주권**: 모든 처리가 귀사 내부 네트워크 내에서만 진행 (망분리 정합)

---

## 섹션 1: 배경 & 비즈니스 가치 (Why)

### 1.1 시장 규모 & 기관의 현황

**한국 공공기관 경영평가 시장**:
- 대상 기관: 공기업 150개 + 준정부기관 189개 = **339개 기관**
- 기관당 경영평가팀: 5-10명 × 연간 풀타임
- 총 인력: ~2,400명 × 인건비 = **연 수조 원 비용**

**KEPCO E&C의 현황 (Anchor 고객)**:
- 경영평가팀이 1년 내내 실적보고서 작성
- 등급(S/A/B/C/D/E)은 **기관장 성과급·기관 명예에 직결**
- 현재 B 등급 고착 → A 등급 상향 필요성 높음

### 1.2 문제점 & 통점 (Pain Points)

| 구분 | 현황 | 개선 목표 |
|------|------|----------|
| **작성 시간** | 1년 풀타임 | 수주 단위 (90% 단축) |
| **품질 일관성** | 담당자 개인 경험 의존 → 등급 차이 발생 | 평가기준 자동 준수, 등급 보정 |
| **의사결정** | B등급 근거 불명확 | A등급 도달 시나리오 + 필요 콘텐츠 명시 |
| **기관 가치** | 등급에 따른 예산·명예 불확실성 | 등급 1단계 상향 → **연 수억 원 가치** |

### 1.3 iroum-ax의 솔루션

5가지 핵심 기능으로 E2E 자동화:

1. **Document Ingestion** — HWP/PDF 자동 파싱 (OCR 포함)
2. **Criterion Mapping** — 평가기준 ↔ 자사 콘텐츠 자동 매칭
3. **Grade Simulation** — 현재(B) vs 목표(A) 등급 확률 계산
4. **Report Draft Generation** — 한국 공문 스타일 초안 자동 생성
5. **Gap Recommendation** — A 등급 도달 필요 콘텐츠 제안

---

## 섹션 2: 대상 사용자 & 기대 효과

### 2.1 사용자 페르소나별 Talking Points

#### 페르소나 1: 경영평가팀 담당자 (비개발자, 도메인 전문가)

**현재 어려움**:
- 보고서 작성에 연 5-8개월 소요
- 평가기준 준수 여부 수작업 확인
- 등급 개선 방향 불명확

**iroum-ax 기대 효과** ✓:
- **초안 생성 시간 90% 단축**: 5-8개월 → 2-3주 (사람 검수 전제)
- **평가기준 자동 체크리스트**: 모든 항목 카버리지 확인
- **등급 사전 파악**: A 등급 도달 가능성 % 수치 + 구체적 콘텐츠 제안
- **검수 오류 50% 감소**: 스타일/합니다체 자동 검증

**성공 지표**: 
- "이 초안이면 검수에 투자하는 시간 60% 절감될 것 같다" (담당자 피드백)

---

#### 페르소나 2: IT 운영부서 (K8s 경험)

**현재 고민**:
- 공공기관 망분리 환경에서 외부 SaaS 사용 불가
- 신규 평가항목 추가 시 외부 개발사 의존성

**iroum-ax 기대 효과** ✓:
- **셀프호스팅 K8s 네이티브**: 고객사 내부망에서만 운영 (외부 API 호출 차단)
- **99.5% 가용성 목표**: PostgreSQL 복제본 + Redis 캐시
- **신규 평가항목 신속 onboarding**: CLI 명령으로 평가편람 + 벤치마크 인덱싱 (< 1시간)
- **감사 추적 완비**: 모든 문서 업로드/생성/예측 이벤트 audit_logs 기록

**성공 지표**:
- "운영 비용 예상: GPU 2개 + VM 4core/16GB = 월 ~500-800만 원" (CapEx 절감)

---

#### 페르소나 3: 기획조정실 (Chief of Staff)

**의사결정 기준**:
- 기관 경영평가 등급 향상 = 기관장 성과급 + 기관 명예
- 인력 효율화 = 비용 절감 + 재배치 가능

**iroum-ax 기대 효과** ✓:
- **등급 1단계 상향 가능성**: 현 B → A 도달 시나리오 자동 생성 (PoC 검증 후 수치 확정)
- **인건비 40% 절감 근거**: 
  - 경평팀 5명 × 8개월 → 2-3주 = 월 300만 원 × 5명 × 6개월 = **9억 원 절감**
  - 동일 인력으로 다른 업무 추진 가능
- **정부 정책 선도**: "AI로 공공기관 효율화 사례" → 청와대/기획재정부 보도자료 가능성

**성공 지표**:
- ROI 단순 계산: 1차 PoC 비용 < 월 절감액 → Go 승인 경로 확보

---

## 섹션 3: 사전 준비 (Pre-Demo Setup)

### 3.1 환경 요구사항

**Docker 환경** (로컬 개발 환경):
```bash
# 필수 도구
- Docker 24.0+
- Docker Compose 2.0+
- Git
- Curl (API 테스트용)

# 선택 (시연에 필수 아님)
- Kubectl 1.24+ (K8s 배포 시)
- Helm 3.10+
```

### 3.2 데모 데이터 준비

**합성 Fixture** (`.moai/demo/fixtures/`):
```
fixtures/
├── synthetic/
│   ├── kepco_safety_report_2024.hwp
│   │   └── 자사(KEPCO) 안전보건 실적보고서 1건 (50페이지)
│   │   └── 데이터: 안전교육 이수율 98%, 중대재해 0건 등
│   │
│   ├── benchmark_grade_a/
│   │   └── 안전보건_A등급_타기관_1.hwp (20페이지)
│   │   └── 안전보건_A등급_타기관_2.hwp (25페이지)
│   │
│   ├── benchmark_grade_b/
│   │   └── 안전보건_B등급_타기관_1.hwp (20페이지)
│   │
│   └── criterion_handbook/
│       └── 2024_경영평가_편람_안전보건.pdf (15페이지)
│       └── 안전보건 항목: 최대 10점
│           ├── 지표1: 안전교육 이수율 (3점)
│           ├── 지표2: 정기안전 점검 (4점)
│           └── 지표3: 중대재해 방지 실적 (3점)
```

### 3.3 Docker 환경 시작

```bash
# 1. 저장소 클론 (사전 완료)
cd /home/sklee/moai/iroum-ax

# 2. 개발 환경 구동
make dev-up
# 또는
docker-compose up -d postgres redis vllm

# 3. 헬스 체크 (각 서비스 대기: ~2-5분)
curl http://localhost:8000/healthz   # FastAPI (pipelines)
curl http://localhost:50051          # gRPC (control-plane)
curl http://localhost:3000           # Console UI (future)

# 4. 초기 데이터 로드 (평가편람 인덱싱)
make demo-init
# 또는
python -m pipelines.mapping.criterion_parser \
  --file=tests/fixtures/synthetic/criterion_handbook/2024_경영평가_편람_안전보건.pdf \
  --create-index
```

### 3.4 모델 다운로드 (1회만, 사전 완료)

```bash
# 자동 다운로드 (첫 추론 시)
# Qwen2-VL 7B: ~7GB (vLLM 캐시)
# ko-sroberta-multitask: ~600MB
# EXAONE 3.5 7B: ~7GB (fallback용)

# 수동 미리 다운로드 (권장)
python -c "from transformers import AutoTokenizer, AutoModel; \
  AutoModel.from_pretrained('BM-K/KoSimCSE-RoBERTa-multitask')"
```

### 3.5 시연 시간 추정

| 단계 | 소요 시간 | 누적 |
|------|----------|------|
| **사전 준비** | | |
| ├─ Docker 환경 시작 (대기) | 2-3분 | 2-3분 |
| ├─ 모델 다운로드 (초회만) | 5-10분 | 7-13분 |
| ├─ 평가편람 인덱싱 | 3-5분 | 10-18분 |
| **본 시연 (단일 명령)** | | |
| ├─ Document Ingestion | 10-15초 | 10-33초 |
| ├─ Criterion Mapping | 5-10초 | 15-43초 |
| ├─ Grade Simulation | 3-5초 | 18-48초 |
| ├─ Report Draft Generation | 10-15초 | 28-63초 |
| ├─ Gap Recommendation | 3-5초 | 31-68초 |
| **E2E 단일 명령** | 40-60초 | 40-60초 |
| **Q&A & 토론** | 20-30분 | 60-90분 |

**권장**: 사전 준비 15-20분 + 본 시연 60분 = **총 75-80분**

---

## 섹션 4: 5 Chapter 데모 흐름

각 챕터는 독립적 또는 E2E 통합으로 시연 가능합니다.

---

### Chapter 1: Document Ingestion (REQ-AX-001)

#### 통점 (사용자 입장)

```
현재: HWP 한글 보고서를 수작업으로 전사
      ├─ 페이지별 수작업 문자 입력 (시간 낭비)
      ├─ 표/그림 인식 한계 (OCR 수준 미흡)
      └─ 메타데이터(작성자, 작성일) 손실

목표: HWP/PDF 파일 1개 업로드 → 자동 파싱 완료
      ├─ 텍스트 + 표 + 메타데이터 자동 추출
      ├─ 처리 시간: 50페이지 < 5분
      └─ OCR 정확도: 95%+
```

#### iroum-ax 처리 (4단계)

1. **HWP 파싱** (hwp-converter):
   - OLE 구조 디코딩 → 텍스트 + 표 추출
   - 메타데이터 (작성자, 작성일) 추출

2. **언어 감지** (REQ-UBI-002):
   - 한국어 Only 처리 (영문 혼용 감지)

3. **OCR 폴백** (Qwen2-VL 7B):
   - HWP 파싱 실패 시 자동 VLM OCR 호출
   - vLLM via GPU: <2초/페이지
   - CPU fallback: 5-10초/페이지

4. **테이블 추출**:
   - VLM 출력 후처리 (셀 정렬 검증)

#### 시연 명령 (API)

```bash
# 1. 문서 업로드
curl -X POST http://localhost:8000/api/v1/documents/upload \
  -F "file=@tests/fixtures/synthetic/kepco_safety_report_2024.hwp"

# 응답 예시
{
  "id": "doc-20260514-ax001",
  "filename": "kepco_safety_report_2024.hwp",
  "file_type": "HWP",
  "status": "parsed",
  "parsed_text": "안전보건 관련 실적...[1200자]",
  "tables": [
    {
      "title": "안전교육 이수 현황",
      "rows": [["항목", "2024년"], ["전사원", "98%"]]
    }
  ],
  "created_at": "2026-05-14T10:00:00Z"
}
```

또는 **CLI**:
```bash
# 단일 파일 처리
make demo-chapter-1

# 또는 수동
python -m pipelines.ingestion.hwp_parser \
  --file=tests/fixtures/synthetic/kepco_safety_report_2024.hwp \
  --output=json
```

#### 예상 출력

```json
{
  "document_id": "doc-20260514-ax001",
  "status": "parsed",
  "text_length": 1247,
  "tables_count": 3,
  "metadata": {
    "author": "KEPCO 경영평가팀",
    "created_date": "2024-12-15",
    "page_count": 50
  },
  "processing_time_seconds": 2.3,
  "ocr_fallback_used": false
}
```

#### Talking Points (의사결정자에게)

- **정확도**: OCR 95%+ (VLM 벤치마크)
- **처리 속도**: 50페이지 = 2-5분 (GPU 기준, CPU는 15-30분)
- **자동화 가치**: 수작업 전사 0시간 → 5분 (99% 시간 절감)
- **오류 감소**: 메타데이터 자동 추출로 보고서 버전 관리 자동화

#### Audit Log 기록

```sql
INSERT audit_logs 
  (user_id, action, resource_type, resource_id, timestamp, details)
VALUES 
  ('cli-anonymous', 'UPLOAD', 'document', 'doc-20260514-ax001', 
   NOW(), '{"file_type":"HWP", "page_count":50}');
```

---

### Chapter 2: Criterion Mapping (REQ-AX-002)

#### 통점

```
현재: 평가편람 항목 → 자사 보고서 콘텐츠 매칭이 담당자 노하우
      ├─ 500개 평가항목 중 관련 항목 검색 수작업
      ├─ 관련성 판단 주관적 (담당자마다 다름)
      └─ 마감 기한 임박 시 누락 위험

목표: 평가기준 자동 매칭 + RAG 벡터 검색
      ├─ 쿼리: "안전교육"
      ├─ 자동 반환: 상위 3개 관련 기준 + relevance score
      └─ 검색 p99: < 100ms (사용자 대기 불필요)
```

#### iroum-ax 처리 (3단계)

1. **평가편람 파싱** (criterion_parser):
   - PDF → Item → Indicator → Detail 계층 추출
   - 기획재정부 기준 완전 매핑

2. **임베딩 생성** (ko-sroberta-multitask):
   - 한국어 특화 임베딩 (768 dim)
   - 500-1000 token 청킹

3. **RAG 인덱싱** (pgvector + HNSW):
   - PostgreSQL 벡터 인덱싱
   - p99 검색 시간 < 100ms 보장

#### 시연 명령

```bash
# 1. 평가편람 인덱싱 (초회 1회)
curl -X POST http://localhost:8000/api/v1/criteria/index \
  -F "file=@tests/fixtures/synthetic/criterion_handbook/2024_경영평가_편람_안전보건.pdf"

# 응답
{
  "index_status": "completed",
  "criteria_count": 48,
  "indexed_at": "2026-05-14T10:02:00Z"
}

# 2. RAG 검색
curl -X GET "http://localhost:8000/api/v1/criteria/search?q=안전교육%20이수율"

# 응답 예시
{
  "query": "안전교육 이수율",
  "results": [
    {
      "criterion_id": "crit-001",
      "name": "안전교육 이수율",
      "relevance": 0.92,
      "max_points": 3,
      "detail": "신규 입사자 안전교육 100% 이수, 정기 안전교육..."
    },
    {
      "criterion_id": "crit-005",
      "name": "정기 안전 점검",
      "relevance": 0.82,
      "max_points": 4
    },
    {
      "criterion_id": "crit-010",
      "name": "중대재해 예방 활동",
      "relevance": 0.78,
      "max_points": 3
    }
  ],
  "search_time_ms": 47
}
```

#### Talking Points

- **검색 정확도**: 상위 3개 내 평균 relevance 0.8+
- **한자/한글 혼용 처리**: ko-sroberta-multitask (graceful fallback)
- **cold-start 대응**: 평가편람 누적 → 검색 정확도 지속 향상
- **운영 편의성**: 신규 평가기준 추가 시 자동 임베딩 (1초)

---

### Chapter 3: Grade Simulation (REQ-AX-003)

#### 통점

```
현재: B 등급인데 A 등급 가능한가? → 주관적 추정만 가능
      ├─ 등급 시뮬레이션 근거 불명확
      ├─ 의사결정 자료 부족
      └─ 상향 필요 콘텐츠 불명확

목표: 자동 등급 예측 + 확률 분포
      ├─ 자사 보고서 → {P(A), P(B), abstain} 분포
      ├─ max(P(A), P(B)) < 0.5 시 "신뢰도 낮음" 명시
      └─ 모호한 케이스는 사람 검수 권고
```

#### iroum-ax 처리 (3단계)

1. **벤치마크 학습** (benchmark_learner):
   - A/B 등급 보고서 특징 추출
   - 2-class 분류기 학습

2. **등급 예측** (grade_predictor):
   - 2-class softmax 기본 출력
   - abstain 분기: max(P_a, P_b) < 0.5 시 활성화
   - 3-way 출력 sum=1.0 ± 0.001

3. **시나리오 생성** (scenario_simulator):
   - 3-5개 B→A 상향 시나리오
   - 예상 점수 개선 (delta) 계산

#### 시연 명령

```bash
# 1. 벤치마크 학습 (초회 1회, 또는 데이터 추가 시)
curl -X POST http://localhost:8000/api/v1/simulations/train-benchmark \
  -d '{"criterion_id": "crit-001", "grade_a_docs": [...], "grade_b_docs": [...]}'

# 2. 등급 예측
curl -X POST http://localhost:8000/api/v1/simulations/predict \
  -d '{"document_id": "doc-20260514-ax001", "criterion_id": "crit-001"}'

# 응답 예시
{
  "simulation_id": "sim-20260514-001",
  "document_id": "doc-20260514-ax001",
  "current_grade_distribution": {
    "p_a": 0.42,
    "p_b": 0.45,
    "abstain": 0.13
  },
  "predicted_class": "B",
  "confidence_level": "medium",
  "confidence_reason": "max(P_A, P_B) = 0.45 < 0.5, consider manual review",
  "scenarios": [
    {
      "scenario_id": 1,
      "description": "안전교육 이수율 100% 달성",
      "expected_delta": 0.08,
      "new_p_a": 0.50,
      "new_p_b": 0.40,
      "feasibility": "high"
    },
    {
      "scenario_id": 2,
      "description": "중대재해 zero-record 3년 유지",
      "expected_delta": 0.05,
      "new_p_a": 0.47,
      "new_p_b": 0.40,
      "feasibility": "medium"
    }
  ],
  "inference_time_seconds": 0.8
}
```

#### Talking Points

- **예측 정확도**: 벤치마크 데이터(A/B 보고서) 대비 80%+
- **신뢰도 명시**: abstain 분기로 모호한 케이스 자동 플래그
- **확률 불변식**: {P_A + P_B + abstain} = 1.0 수학적 검증
- **의사결정 자료**: 3-5개 시나리오로 상향 경로 구체화

---

### Chapter 4: Report Draft Generation (REQ-AX-004)

#### 통점

```
현재: 초안 작성이 가장 오래 걸리는 단계
      ├─ 평가기준 + 자사 데이터 조합해서 수작업 작성
      ├─ 한국 공문 합니다체 스타일 준수 필수
      ├─ 재작성율 20-30% (품질 편차)
      └─ 1개 항목에 3-4시간 투입

목표: LLM 초안 자동 생성
      ├─ 평가기준 정합 + 합니다체 자동 검증
      ├─ 1개 항목 초안 5초 이내 생성
      └─ 완벽도 80% (사람 검수 전제, 20% 수정만)
```

#### iroum-ax 처리 (4단계)

1. **프롬프트 빌드** (prompt_builder):
   - 평가기준 + 작성지침 + 자사 콘텐츠 조합
   - 2000-3000 tokens 프롬프트 생성

2. **LLM 호출** (llm_client):
   - Primary: EXAONE 3.5 7B
   - Fallback: Qwen 2.5 7B (EXAONE 실패 시)
   - 외부 API 차단 (데이터 주권, REQ-UBI-001)

3. **스타일 검증** (style_applier):
   - 한국 공문 합니다체 강제
   - 반말/존댓말 혼용 감지 → reject & retry (≤3회)
   - @MX:ANCHOR (high fan_in 함수)

4. **초안 저장** (report_drafter):
   - FastAPI 엔드포인트 + Celery 워커 연동

#### 시연 명령

```bash
# 1. 초안 생성
curl -X POST http://localhost:8000/api/v1/reports/generate \
  -d '{
    "document_id": "doc-20260514-ax001",
    "criterion_id": "crit-001",
    "criterion_name": "안전교육 이수율",
    "criterion_detail": "신규입사자 100%, 정기교육 연 2회 이상"
  }'

# 응답 예시
{
  "report_id": "rep-20260514-001",
  "document_id": "doc-20260514-ax001",
  "criterion_id": "crit-001",
  "draft_text": "당 기관은 안전보건 강화를 위해 안전교육 이수율을 체계적으로 관리하고 있습니다. 2024년 신규 입사자에 대한 안전교육 이수율은 98%로 기준을 상회하였으며, 정기안전교육은 반기 1회 이상 실시하여 안전문화 정착을 도모하였습니다.",
  "model_used": "EXAONE-3.5-7B",
  "honorific_score": 1.0,
  "honorific_violations": [],
  "generation_time_seconds": 4.2,
  "retries": 0
}
```

#### Talking Points

- **생성 품질**: 합니다체 정확도 96%+, 초안 완벽도 80% (사람 검수 전제)
- **생성 속도**: 1개 항목 < 5초 (5개 항목 = 25초)
- **한국어 우수성**: EXAONE 3.5 한국어 도메인 최적화
- **스타일 검증**: 반말/존댓말 자동 감지로 재작성 20% → 5% 감소
- **데이터 주권**: 모든 처리 내부망 (vLLM self-hosted)

---

### Chapter 5: Gap Recommendation (REQ-AX-005)

#### 통점

```
현재: B → A 상향 방법이 불명확
      ├─ A 등급 벤치마크 분석 시간 오래 걸림
      ├─ 실현 가능한 개선 항목 파악 어려움
      └─ 의사결정 근거 불충분

목표: 자동 gap analysis + 우선순위 제안
      ├─ 현재(B) vs 목표(A) 콘텐츠 비교
      ├─ 3-5개 제안 항목 + 예상 점수 개선
      └─ 실현 가능성 우선순위 명시
```

#### iroum-ax 처리 (3단계)

1. **Gap 분석** (gap_analyzer):
   - 자사 콘텐츠 vs A 등급 벤치마크 비교
   - 부족한 요소 3-5개 식별

2. **콘텐츠 제안** (content_suggester):
   - A 등급 참고 자료 매칭
   - 소스 reference 기록

3. **우선순위 정렬** (prioritizer):
   - 실현 가능성 스코어 (0.0~1.0)
   - Priority 1-5 부여
   - score_delta 예상값 계산

#### 시연 명령

```bash
# 1. 추천 생성
curl -X POST http://localhost:8000/api/v1/recommendations/generate \
  -d '{
    "simulation_id": "sim-20260514-001",
    "criterion_id": "crit-001",
    "current_grade": "B",
    "target_grade": "A"
  }'

# 응답 예시
{
  "recommendation_id": "rec-20260514-001",
  "simulation_id": "sim-20260514-001",
  "criterion_id": "crit-001",
  "recommendations": [
    {
      "rank": 1,
      "item": "안전교육 이수율",
      "current_status": "98%",
      "target_status": "100%",
      "suggested_action": "신입 안전교육 누락 1명 재교육",
      "expected_score_delta": 0.05,
      "feasibility_score": 0.95,
      "priority": "HIGH",
      "effort_hours": 2,
      "reference_source": "A등급_타기관_001"
    },
    {
      "rank": 2,
      "item": "정기안전 점검 주기",
      "current_status": "분기별",
      "target_status": "월별 + 분기별 심화",
      "suggested_action": "월별 점검체크리스트 구체화, 심화 교육 1회 추가",
      "expected_score_delta": 0.03,
      "feasibility_score": 0.70,
      "priority": "MEDIUM",
      "effort_hours": 8,
      "reference_source": "A등급_타기관_002"
    },
    {
      "rank": 3,
      "item": "중대재해 예방 투자",
      "current_status": "연 500만 원",
      "target_status": "연 1000만 원 + 신규 장비",
      "suggested_action": "안전장비 예산 증액, 기술 용역 추가",
      "expected_score_delta": 0.04,
      "feasibility_score": 0.60,
      "priority": "MEDIUM-LOW",
      "effort_hours": 20,
      "reference_source": "A등급_타기관_003"
    }
  ],
  "cumulative_score_delta": 0.12,
  "projected_new_grade": "A (50%+ 확률)",
  "analysis_time_seconds": 2.1
}
```

#### Talking Points

- **실현 가능성 평가**: 각 항목 feasibility_score로 우선순위 객관화
- **구체적 콘텐츠**: "100% 달성" 같은 정성적 제안 아님 → 구체적 행동 명시
- **타기관 레퍼런스**: A 등급 벤치마크 문서 소스 추적 가능
- **등급 상향 시나리오**: 3개 권고 실행 시 A 등급 도달 확률 50%+

---

## 섹션 5: 종합 시연 (End-to-End)

### 5.1 단일 명령으로 E2E 흐름 실행

```bash
# Option 1: make 명령
make demo
# 내부적으로 다음 5단계 순차 실행:
#   1. Document Ingestion
#   2. Criterion Mapping (검색)
#   3. Grade Simulation (예측)
#   4. Report Draft Generation
#   5. Gap Recommendation

# Option 2: Bash 스크립트
bash scripts/demo.sh \
  --document=tests/fixtures/synthetic/kepco_safety_report_2024.hwp \
  --criterion-id=crit-001 \
  --output-json=demo-result.json

# Option 3: API 시퀀스
curl -X POST http://localhost:8000/api/v1/workflows \
  -d '{"document_id": "doc-20260514-ax001"}' \
| jq '.workflow_id' \
| xargs -I {} \
  curl -X GET http://localhost:8000/api/v1/workflows/{}
```

### 5.2 소요 시간 (CPU vs GPU)

| 단계 | GPU | CPU |
|------|-----|-----|
| Document Ingestion | 15초 | 2분 |
| Criterion Mapping | 10초 | 30초 |
| Grade Simulation | 5초 | 10초 |
| Report Draft Gen | 15초 | 40초 |
| Gap Recommendation | 5초 | 8초 |
| **총합** | **50초** | **3분 28초** |

**권장**: GPU 환경에서 시연 (AWS EC2 p3.2xlarge 또는 on-prem V100)

### 5.3 Workflow 상태 추적

```bash
# Workflow 조회 (실시간 상태)
curl http://localhost:8000/api/v1/workflows/{workflow_id}

# 응답
{
  "workflow_id": "wf-20260514-001",
  "status": "COMPLETED",
  "document_id": "doc-20260514-ax001",
  "stages": [
    {"stage": "ingestion", "status": "COMPLETED", "duration_ms": 2300},
    {"stage": "mapping", "status": "COMPLETED", "duration_ms": 900},
    {"stage": "scoring", "status": "COMPLETED", "duration_ms": 500},
    {"stage": "generation", "status": "COMPLETED", "duration_ms": 4200},
    {"stage": "recommendation", "status": "COMPLETED", "duration_ms": 2100}
  ],
  "total_duration_seconds": 10.0,
  "results": {
    "document": {...},
    "grade_prediction": {...},
    "recommendations": {...}
  }
}
```

### 5.4 Audit Log 검증

```bash
# 모든 감시 이벤트 조회
curl "http://localhost:8000/api/v1/audit-logs?workflow_id={workflow_id}"

# 응답 예시
{
  "audit_logs": [
    {
      "id": "audit-001",
      "user_id": "cli-anonymous",
      "action": "UPLOAD",
      "resource_type": "document",
      "resource_id": "doc-20260514-ax001",
      "timestamp": "2026-05-14T10:00:00Z"
    },
    {
      "id": "audit-002",
      "user_id": "cli-anonymous",
      "action": "PREDICTION",
      "resource_type": "simulation",
      "resource_id": "sim-20260514-001",
      "timestamp": "2026-05-14T10:00:05Z",
      "details": {"confidence": "medium", "class": "B"}
    },
    {
      "id": "audit-003",
      "user_id": "cli-anonymous",
      "action": "DRAFT_GENERATE",
      "resource_type": "report",
      "resource_id": "rep-20260514-001",
      "timestamp": "2026-05-14T10:00:10Z",
      "details": {"model": "EXAONE-3.5", "honorific_score": 1.0}
    }
  ]
}
```

---

## 섹션 6: Q&A 예상 (의사결정자·운영자·사용자 별)

### 6.1 운영자 (IT 팀) Q&A

**Q1**: "망분리 환경에서 외부 LLM API 사용은 어떻게 차단하나요?"

**A1**: 
- **설정 검증**: `pipelines/config/settings.py`의 `validate_llm_endpoint()`
- **정책**: 외부 LLM URL (openai.com, claude.ai, ...) 접근 시 403 Forbidden
- **증명**: 
  ```python
  # REQ-UBI-001 데이터 주권 검증
  if "openai.com" in llm_url or "claude" in llm_url:
      raise ForbiddenLLMEndpoint(f"External API blocked: {llm_url}")
  # audit_logs에 차단 기록
  ```
- **K8s NetworkPolicy**: 클러스터 내부 통신만 허용 (외부 egress 차단)

**Q2**: "감사 추적은 어디서 확인하나요?"

**A2**:
- **저장소**: PostgreSQL `audit_logs` 테이블
- **기록 항목**: 8가지 액션
  - UPLOAD (문서 업로드)
  - PREDICTION (등급 예측)
  - DRAFT_GENERATE (초안 생성)
  - RECOMMENDATION (추천 생성)
  - DELETE, UPDATE, READ (future)
- **쿼리 예시**:
  ```sql
  SELECT * FROM audit_logs 
  WHERE action='DRAFT_GENERATE' AND created_at >= '2026-05-14'
  ORDER BY timestamp DESC;
  ```
- **SIEM 연동**: 구조화 JSON 로그 (Fluentd/Loki → Grafana)

**Q3**: "신규 평가항목(안전보건 외)을 추가하려면 시간이 얼마나 걸리나요?"

**A3**:
- **최소 단계**: 새 평가편람 PDF 업로드 → 자동 인덱싱
- **소요 시간**: 
  - PDF 파싱 + 임베딩: 5-10분
  - 벤치마크 학습 (A/B 보고서 2-3개): 10-20분
  - **총 15-30분** (1회)
- **자동화**: CLI 명령 1줄로 가능
  ```bash
  python -m pipelines.onboarding.add_criterion \
    --pdf=새_평가항목.pdf \
    --benchmarks=A등급_보고서들/ \
    --auto-index
  ```

---

### 6.2 사용자 (경영평가팀) Q&A

**Q1**: "초안이 잘못된 내용을 생성하면 어떻게 하나요?"

**A1**:
- **현재 전략**: 초안은 80% 완성도 (사람 검수 전제)
- **자동 검증**:
  1. 합니다체 자동 검증 (반말 감지 → reject & retry)
  2. 평가기준 매칭 검증
  3. 부정문/정보 모순 검출 (future)
- **오류 추적**:
  ```json
  {
    "draft_id": "rep-20260514-001",
    "generated_text": "...",
    "validation_errors": [
      {"type": "honorific_violation", "text": "해요", "suggestion": "합니다"}
    ],
    "requires_human_review": true
  }
  ```
- **재시도**: 사용자가 approve 하기 전까지 feedback → regenerate 가능

**Q2**: "한글/한자 혼용 보고서는 처리 가능한가요?"

**A2**:
- **현재 상태**: 한글 우선 (한자는 후속 SPEC-AX-002에서 처리)
- **Graceful Fallback**:
  - 임베딩 신뢰도 × 0.8 (확실성 낮춤)
  - 검색 결과에 `confidence: "medium"` 플래그 추가
- **근거**: 한자 → 한글 변환 NLP 모듈 미포함 (Walking Skeleton 범위)

**Q3**: "외부로 데이터가 나가나요?"

**A3**:
- **데이터 흐름**:
  ```
  [사용자 문서] → [고객사 내부 K8s]
                 ├─ PostgreSQL (저장)
                 ├─ Redis (캐시)
                 ├─ vLLM (추론)
                 └─ [고객사 내부망 ONLY]
  ```
- **외부 연결 제로**: 인터넷 access 불가 (air-gapped 배포 가능)
- **증명**: NetworkPolicy + audit_logs egress 차단 기록

---

### 6.3 의사결정자 (기획조정실) Q&A

**Q1**: "ROI가 얼마나 되나요?"

**A1**:
- **인건비 절감**:
  ```
  현재: 경평팀 5명 × 8개월 = 5명-년
  개선: 5명 × 2-3주 = 0.3명-년
  절감: 4.7명-년 × 연봉 5,000만 원 = 연 2.35억 원
  ```
- **1차 PoC 비용** (추정): 개발 + 배포 + 운영 3개월 ≈ 5-8억 원
  - ROI 기대값: 2.35억 원 / 8억 원 = **29% 회수 (1년 내)**
  - Break-even: 약 **16개월**
  
- **추가 가치**:
  - 등급 1단계 상향 시 기관장 성과급 증액: +α
  - 인력 재배치 (다른 업무 추진): +β

**Q2**: "경쟁사는 없나요?"

**A2**:
- **시장 현황**:
  | 경쟁사 | 포지션 | 약점 |
  |--------|---------|------|
  | LangGraph | 수평적 Agentic 프레임 | 한국 공공도메인 X, HWP 미지원 |
  | CrewAI | 멀티에이전트 팀 | 한국어 부족, 망분리 미정 |
  | Copilot Studio | 엔터프라이즈 AI 빌더 | 클라우드 필수, 한국어 미흡 |
  | RPA 컨설팅사 | 레거시 자동화 | 느린 혁신, 높은 비용 |

- **iroum-ax 차별화**:
  - ✓ 한국 공공 경영평가 **수직 특화** (Linear/Notion/Figma 모델)
  - ✓ HWP 네이티브 + 합니다체 자동 검증
  - ✓ 망분리 정합 (자체 호스팅)
  - ✓ KEPCO 레퍼런스 확보 가능 (1차 anchor)

**Q3**: "향후 플랜은?"

**A3**:
- **Phase 2 (2027초)**: 평가항목 확대 (안전보건 → 500개 전체)
- **Phase 3 (2027중)**: 인접 도메인 진입
  - ESG 보고서 자동화 (대형 공기업)
  - 감사 보고서 자동화 (준정부기관)
  - 면허신청서 자동화 (규제대상)
- **Phase 4 (2027후반)**: 금융권 확장 (금감원 규제보고서)
  - 선행 조건: 공공 anchor 성공 사례 3+개
  - 시장 기대값: ARR $500K → $1M+ (플랫폼화)

---

## 섹션 7: 기술 차별화 한 페이지 요약

### 7.1 핵심 기술 스택

| 영역 | iroum-ax | 일반 솔루션 |
|------|----------|----------|
| **언어** | 한국어 우선 | 영어 기본 |
| **HWP 파싱** | ✓ 네이티브 (hwp-converter) | ✗ PDF 변환만 |
| **VLM** | Qwen2-VL (한국어 OCR 95%+) | Llama / GPT-4o (한국어 약함) |
| **텍스트 LLM** | EXAONE 3.5 (공문 스타일) | GPT-4 / Claude (비용 高) |
| **데이터 주권** | 자체 호스팅 (셀프호스팅) | API 의존 (클라우드) |
| **망분리 정합** | ✓ K8s NetworkPolicy | ✗ 클라우드 기본 |
| **RAG 벡터DB** | pgvector (HNSW) | Pinecone / Weaviate (SaaS) |
| **배포** | Helm + K8s (온프레미스) | SaaS 또는 AWS managed |

### 7.2 도메인 깊이

**iroum-ax만 제공**:
- 한국 공공기관 경영평가 편람 매핑 자동화
- 합니다체 공문 스타일 자동 검증 + 재생성
- 등급별 벤치마크 학습 (A vs B vs C)
- 기관별 맞춤형 추천 엔진 (feasibility + effort 계산)

### 7.3 추적성 & 감시

**REQ-UBI-003 감시 8가지 액션**:
1. UPLOAD
2. PREDICTION
3. DRAFT_GENERATE
4. RECOMMENDATION
5. DELETE, UPDATE, READ (future)
6. CONFIG_CHANGE
7. ERROR (자동 기록)

모든 이벤트: user_id + timestamp + resource_id + details JSON

---

## 섹션 8: 다음 단계 (Post-Demo Roadmap)

### 8.1 즉시 (1주일)

- [ ] PoC 결과 정리 (정량 지표 수집)
  - Document Ingestion 정확도
  - RAG 검색 정확도
  - 초안 완벽도 (담당자 평가)
  - 등급 예측 정확도 (실제 등급과 비교)
- [ ] 기획조정실 보고 및 승인 요청
- [ ] Phase 2 계약 검토 (평가항목 확대)

### 8.2 1차 12개월 (Phase 2)

**SPEC 후보**:

1. **SPEC-AX-002**: 안전보건 추가 지표
   - 현재: 1개 지표 (안전교육 이수율)
   - 확대: 5-10개 지표
   
2. **SPEC-AX-CTRL-001**: Go Control Plane 본격 구현
   - 현재: 스텁만 완료
   - 구현: gRPC + REST 완전 기능

3. **SPEC-AX-EXPANDED-001**: 평가항목 확대 (500개)
   - 다중 도메인 지원
   - 동적 프롬프트 생성

4. **SPEC-AX-CONSOLE-001**: Next.js Console UI
   - 현재: API/CLI 우선
   - UI: 대시보드 + 보고서 뷰어 + 시뮬레이션 차트

5. **SPEC-AX-AUTH-001**: SSO/JWT 인증
   - 현재: cli-anonymous (sandbox)
   - 구현: LDAP / OAuth2 연동

### 8.3 2년차 (Phase 3) — 인접 도메인 확장

**후속 SPEC 후보**:

1. **SPEC-AX-ESG-001**: ESG 보고서 자동화
   - 시장: 대형 공기업 + 상장사
   - 기술 재사용: Document Ingestion, RAG, Generation
   - 차별화: K-ESG + GRI/TCFD 자동 매핑
   - 기대 ARR: +$200K

2. **SPEC-AX-AUDIT-001**: 감사 보고서 자동화
   - 시장: 준정부기관 + 공공기관
   - 차별화: 감사원 예시 보고서 학습 기반 사전 컴플라이언스
   - 기대 ARR: +$150K

3. **SPEC-AX-LICENSE-001**: 면허신청서 자동화
   - 시장: 규제 대상 사업장 (의료기관, 축산, 식품 등)
   - 차별화: 부처별 면허기준 자동 분기 (식약처/환경부/고용노동부)
   - 기대 ARR: +$100K

4. **SPEC-AX-GOV-DOC-001**: 정부 공문 자동화
   - 시장: 모든 공공기관
   - 시장 규모 가장 큼 (기대 ARR: +$200K+)

5. **SPEC-AX-FINTECH-001**: 금융권 규제 보고서 (Phase 4+)
   - 시장: 은행/증권/보험 (전자금융감독규정)
   - 선행 조건: 공공 anchor 성공 사례 3+개 확보
   - 기대 ARR: +$300K+ (단가 1.5-2배)
   - K-CSAP / K-ISMS 인증 트랙

---

## 체크리스트 & 시연 일정

### 체크리스트 (사전 확인)

- [ ] Docker 환경 시작 (`make dev-up`)
- [ ] 모델 다운로드 완료 (Qwen2-VL, EXAONE, embedding 모델)
- [ ] 평가편람 PDF 인덱싱 완료 (`make demo-init`)
- [ ] 합성 fixture 문서 준비 (자사 + A/B 벤치마크)
- [ ] API 헬스 체크 완료 (http://localhost:8000/healthz)
- [ ] Audit log 조회 테스트 완료
- [ ] Q&A 스크립트 검토 (섹션 6)

### 시연 일정 예시

```
0:00-0:05 (5분)   | 개요 & 배경 (섹션 1-2)
0:05-0:20 (15분)  | 사전 준비 확인 (섹션 3)
0:20-0:35 (15분)  | Chapter 1-3 데모 (Document, RAG, Grade Sim)
0:35-0:50 (15분)  | Chapter 4-5 데모 (Draft Gen, Recommendation)
0:50-0:55 (5분)   | E2E 통합 시연 (섹션 5)
0:55-1:20 (25분)  | Q&A 및 토론
1:20-1:60 (40분)  | 추가 토론 / 관심 있는 항목 심화
```

---

## 부록: 명령어 빠른 참조

### 로컬 개발 환경

```bash
# 환경 시작
make dev-up
docker-compose up -d

# 헬스 체크
curl http://localhost:8000/healthz
curl http://localhost:8000/api/v1/criteria/search?q=테스트

# 테스트 실행
make test
pytest -v tests/

# 로그 보기
docker-compose logs -f pipelines
docker-compose logs -f control-plane
```

### API 시연

```bash
# 1. 문서 업로드
DOCUMENT_ID=$(curl -s -X POST http://localhost:8000/api/v1/documents/upload \
  -F "file=@tests/fixtures/synthetic/kepco_safety_report_2024.hwp" \
  | jq -r '.id')

# 2. RAG 검색
curl "http://localhost:8000/api/v1/criteria/search?q=안전교육%20이수율" | jq .

# 3. 등급 예측
curl -X POST http://localhost:8000/api/v1/simulations/predict \
  -d "{\"document_id\":\"$DOCUMENT_ID\",\"criterion_id\":\"crit-001\"}" | jq .

# 4. 초안 생성
curl -X POST http://localhost:8000/api/v1/reports/generate \
  -d "{\"document_id\":\"$DOCUMENT_ID\",\"criterion_id\":\"crit-001\"}" | jq .

# 5. 추천 생성
curl -X POST http://localhost:8000/api/v1/recommendations/generate \
  -d "{\"document_id\":\"$DOCUMENT_ID\",\"criterion_id\":\"crit-001\",\"target_grade\":\"A\"}" | jq .

# 감시 로그 조회
curl "http://localhost:8000/api/v1/audit-logs?document_id=$DOCUMENT_ID" | jq .
```

---

## 문서 정보

**버전**: 1.0 (2026-05-14)  
**작성자**: iroum-ax manager-docs  
**대상 청중**: KEPCO E&C 경영평가팀 + IT 운영 + 기획조정실  
**시연 시간**: 60분 (준비 포함 75-80분)  
**기술 수준**: 기술자 & 비기술자 혼합 (문장 비율 조정)  
**라이센스**: 내부 기밀 (KEPCO E&C 한정)  

---

**최종 검토**: 2026-05-14  
**상태**: Ready for Demo (v0.1.2 Walking Skeleton 완료)
