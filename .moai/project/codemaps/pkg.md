# 공유 라이브러리 맵 (pkg)

5개 Pydantic 데이터 모델 + 에러 정의 + 로깅 구조

## 공유 모델 (pkg/models/)

### 1. Document Model

```python
# pkg/models/document.py

class Document(BaseModel):
    id: UUID
    filename: str
    file_type: Literal['HWP', 'PDF', 'IMAGE']
    parsed_text: str
    tables: List[Dict[str, Any]]  # VLM 후처리 결과
    metadata: DocumentMetadata
    status: Literal['parsed', 'ocr_fallback', 'parse_failed']
    created_at: datetime
    updated_at: datetime

class DocumentMetadata(BaseModel):
    author: str
    created_date: date
    organization: str
    language: Literal['ko', 'en', 'mixed']
```

**사용처**: ingestion 모듈 · control-plane workflow storage

---

### 2. Criterion Model

```python
# pkg/models/criterion.py

class Criterion(BaseModel):
    id: UUID
    criterion_name: str  # 예: "안전교육 이수율"
    criterion_detail: str
    max_points: int
    hierarchy_level: int  # 0=항목, 1=지표, 2=배점
    parent_id: Optional[UUID]  # 계층 구조
    embedding: List[float]  # 768 dim (ko-sroberta-multitask)
    relevance_score: float  # 검색 시 계산됨

class CriterionHierarchy(BaseModel):
    items: List[Criterion]
    depth: int
    leaf_count: int
```

**사용처**: mapping 모듈 · pgvector 인덱싱 · RAG 검색

---

### 3. Report Model

```python
# pkg/models/report.py

class Report(BaseModel):
    id: UUID
    organization_name: str
    document_id: UUID  # 소스 Document 참조
    grade: Literal['A', 'B', 'C', 'D']
    score: int  # 0-100
    content: str  # 보고서 텍스트
    sections: List[ReportSection]
    metadata: ReportMetadata
    created_at: datetime
    updated_at: datetime

class ReportSection(BaseModel):
    criterion_id: UUID
    section_title: str
    content: str
    confidence_score: float  # LLM 신뢰도

class ReportMetadata(BaseModel):
    is_benchmark: bool  # A/B 등급 벤치마크 보고서 여부
    benchmark_type: Optional[Literal['A', 'B', 'C', 'D']]
    generation_model: str  # EXAONE 3.5 또는 Qwen 2.5
    style_validation_passed: bool
```

**사용처**: scoring·generation·recommendation 모듈 · PostgreSQL reports 테이블

---

### 4. Simulation Model

```python
# pkg/models/simulation.py

class SimulationResult(BaseModel):
    id: UUID
    workflow_id: UUID
    document_id: UUID
    current_grade: Literal['A', 'B', 'C', 'D']
    target_grade: Literal['A', 'B', 'C', 'D']
    probabilities: GradeProbabilities
    abstain_flag: bool  # 두 클래스 모두 < 0.5 시 True
    confidence_level: Literal['high', 'medium', 'low']
    scenarios: List[ScenarioResult]
    created_at: datetime

class GradeProbabilities(BaseModel):
    A: float  # 0.0~1.0
    B: float
    abstain: float  # max(A, B) < 0.5 일 때만 활성화
    total: float  # = 1.0

class ScenarioResult(BaseModel):
    scenario_name: str  # 예: "안전교육 이수율 5% 증가"
    estimated_score_delta: int  # 가산점
    new_estimated_grade: Literal['A', 'B', 'C', 'D']
    probability: float
```

**사용처**: scoring 모듈 · PostgreSQL simulations 테이블 · recommendation 입력

---

### 5. Recommendation Model

```python
# pkg/models/recommendation.py

class Recommendation(BaseModel):
    id: UUID
    simulation_id: UUID
    criterion_id: UUID
    item_title: str  # 예: "안전교육 이수율 → 목표 Y% 상향"
    current_value: str  # 현재 상태
    recommended_value: str  # 목표 상태
    estimated_score_delta: int  # 기대 개선점
    feasibility_score: float  # 0.0~1.0 (실현 가능성)
    priority: int  # 1=최고, 3-5=중간
    reason: str  # 왜 이 항목을 제안했는지
    reference_sources: List[str]  # 벤치마크 보고서 ID
    feedback_status: Literal['pending', 'accepted', 'not_feasible', 'implemented']
    created_at: datetime

class RecommendationBatch(BaseModel):
    simulation_id: UUID
    items: List[Recommendation]
    total_score_improvement: int  # 전체 기대 개선점
```

**사용처**: recommendation 모듈 · PostgreSQL recommendations 테이블 · 최종 API 응답

---

## 에러 정의 (pkg/errors/)

```python
# pkg/errors/custom_errors.py

class IroumAxError(Exception):
    """기본 에러 클래스"""
    error_code: str
    error_message: str

# Document Ingestion 에러
class HWPParseError(IroumAxError):
    error_code = "HWP_PARSE_001"
    # OLE 구조 손상, 인코딩 이슈 등

class PDFParseError(IroumAxError):
    error_code = "PDF_PARSE_001"

class VLMOCRError(IroumAxError):
    error_code = "VLM_OCR_001"
    # GPU 메모리 부족, 모델 로딩 실패

# Mapping 에러
class RAGInsufficientContextError(IroumAxError):
    error_code = "RAG_INSUFFICIENT_001"
    # top-3 검색 결과가 relevance < 0.7

class EmbeddingDimensionError(IroumAxError):
    error_code = "EMBEDDING_DIM_001"
    # 768 vs 1536 불일치

# Scoring 에러
class BenchmarkNotAvailableError(IroumAxError):
    error_code = "BENCHMARK_NA_001"
    # A/B 등급 벤치마크 데이터 부재

class LowConfidencePredictionError(IroumAxError):
    error_code = "LOW_CONFIDENCE_001"
    # abstain 플래그 활성화

# Generation 에러
class LLMGenerationError(IroumAxError):
    error_code = "LLM_GEN_001"

class KoreanStyleViolationError(IroumAxError):
    error_code = "STYLE_VIOLATION_001"
    # 공문 합니다체 위반

# Data Sovereignty 에러
class DataSovereigntyError(IroumAxError):
    error_code = "DATA_SOVEREIGNTY_001"
    # 외부 LLM API 호출 시도 감지
```

**사용처**: 모든 모듈 · 에러 핸들링 · audit_logs 기록

---

## 로깅 구조 (pkg/logging/)

```python
# pkg/logging/logger.py

class AuditLogger:
    """REQ-UBI-003 감시 로그 기록"""
    
    def log_document_upload(
        self, user_id: str, document_id: UUID, filename: str
    ) -> None:
        # audit_logs 테이블에 기록
        # action='UPLOAD', resource_type='document', resource_id=document_id
    
    def log_workflow_created(
        self, user_id: str, workflow_id: UUID, document_id: UUID
    ) -> None:
        # action='WORKFLOW_CREATE'
    
    def log_draft_generated(
        self, user_id: str, report_id: UUID, model_name: str
    ) -> None:
        # action='DRAFT_GENERATE', details={'model': model_name}
    
    def log_prediction_made(
        self, user_id: str, simulation_id: UUID, confidence: str
    ) -> None:
        # action='PREDICTION', details={'confidence': confidence}

class StructuredLogger:
    """모든 로그는 JSON 구조화"""
    
    def info(self, message: str, **context) -> None:
        # {
        #   "timestamp": "2026-05-14T10:30:45Z",
        #   "level": "INFO",
        #   "service": "pipelines",
        #   "message": message,
        #   **context
        # }
```

**사용처**: FastAPI middleware · Celery 워커 · 모든 진입점

---

## 모델 간 관계

```
Document ──→ (파싱) ──→ Criterion (RAG 인덱싱)
    │
    └──→ (벤치마크) ──→ Report
            │
            └──→ (학습) ──→ SimulationResult
                      │
                      └──→ (분석) ──→ Recommendation
```

---

**최종 업데이트**: 2026-05-14 (Sprint 6 완료)  
**모델 총 5개**: Document · Criterion · Report · Simulation · Recommendation  
**에러 클래스 총 10개**: 도메인별 분류됨  
**Audit 대상**: REQ-UBI-003 compliance
