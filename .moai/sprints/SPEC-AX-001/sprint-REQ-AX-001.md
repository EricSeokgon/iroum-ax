# Sprint Contract — REQ-AX-001 Document Ingestion

- SPEC: SPEC-AX-001 v0.1.2
- Sprint: 2 (REQ-AX-001)
- Harness level: thorough
- Priority dimension: **Functionality** (파싱 정확도 95% + 폴백 체인이 파이프라인 진입점)
- 작성일: 2026-05-14
- 상태: RED phase 완료

---

## Acceptance Checklist

| AC | 설명 | 상태 |
|----|------|------|
| AC-001-1 | 정상 HWP 파싱 — ParsedDocument 반환 (30s 이내, 메타데이터 + 텍스트 + 테이블) | RED |
| AC-001-2 | HWP OLE 구조 손상 → VLM OCR 자동 폴백 (status=`ocr_fallback`, 사용자 개입 없음) | RED |
| AC-001-3 | 회전된 PDF 표 페이지 — 셀 논리적 행/열 순서 보존 + rotation 메타데이터 기록 | RED |
| AC-001-4 | 동일 문서 OCR 동시 요청 — GPU memory race 방지 큐잉 (HTTP 202 또는 409) | RED |
| AC-001-5 | GPU/CPU 분기 추적 가능성 — `inference_backend` 메타데이터 반영 + CPU p99 < 20s/page | RED |

---

## Priority Dimension: Functionality

REQ-AX-001은 전체 파이프라인 진입점이다. 문서 수집 실패는 REQ-AX-002~005 모두를 차단한다.

Evaluator 4-dim 가중치:
- **Functionality**: 40% (파싱 정확도 95%, 폴백 체인)
- **Originality**: 25% (한국어 HWP + VLM OCR 특화)
- **Completeness**: 20% (5개 AC 모두 커버)
- **Security**: 15% (OCR 폴백 시 audit log 기록)

---

## Test Scenarios (Playwright → httpx/FastAPI testclient 대체)

Console UI 제외 (SPEC-AX-001 Exclusion §1)에 따라 Playwright는 httpx + FastAPI testclient로 대체한다.

### 시나리오 1: HWP 정상 파싱

```
Given: 합성 5페이지 안전보건 HWP 파일 (tests/fixtures/synthetic/sample_report.hwp)
       + HWPParser가 임포트 가능
When:  parser.parse(file_path) 호출
Then:  ParsedDocument 반환
       - text: str (비어 있지 않음)
       - tables: list[Table] (≥ 0)
       - metadata.author: str
       - metadata.created_date: str
```

### 시나리오 2: OLE 손상 폴백

```
Given: 손상된 HWP 파일 (tests/fixtures/synthetic/corrupted_ole.hwp)
       + VLMProcessor가 mock으로 대체
When:  HWPParser.parse() 호출 시 OLE 오류 발생
Then:  자동으로 VLMProcessor.ocr() 호출
       - 반환된 ParsedDocument.status == "ocr_fallback"
       - 사용자 개입 없음
```

### 시나리오 3: 회전 PDF 표 추출

```
Given: 90도 회전된 표 페이지 포함 PDF (tests/fixtures/synthetic/rotated_table.pdf)
When:  PDFParser.parse() → table_extractor.extract()
Then:  Table.rotation == 90
       Table.cells 논리적 순서 (행/열)
```

### 시나리오 4: GPU/CPU 분기

```
Given: VLMProcessor(use_gpu=False)  [CPU 환경]
       VLMProcessor(use_gpu=True)   [GPU 환경, @pytest.mark.gpu로 opt-in]
When:  vlm_processor.ocr(image_path) 호출
Then:  반환 메타데이터에 model_used 또는 inference_backend 필드 존재
       CPU: inference_backend == "transformers_cpu"
       GPU: inference_backend == "vllm_gpu"  (@pytest.mark.gpu)
```

---

## Pass Conditions (GREEN phase 기준)

- 5개 AC 시나리오 모두 자동화 테스트 통과
- 단위 + 통합 coverage ≥ 85%
- HWP OLE 손상 복원율 ≥ 80% (합성 10건 샘플 — GREEN phase에서 측정)
- OCR p99 < 20s/page (CPU 환경, 5-10× 완화 per tech.md §6.1)
- OCR p99 < 2s/page (GPU 환경, @pytest.mark.gpu, opt-in)
- LSP errors=0, type_errors=0, lint_errors=0

---

## Test Interface Contracts (RED phase에서 테스트 대상)

구현 없이 테스트가 참조하는 인터페이스:

```python
# pipelines.ingestion.hwp_parser
class HWPParser:
    def parse(self, file_path: str) -> ParsedDocument: ...

# pipelines.ingestion.pdf_parser
class PDFParser:
    def parse(self, file_path: str) -> ParsedDocument: ...

# pipelines.ingestion.vlm_processor
class VLMProcessor:
    def __init__(self, use_gpu: bool = False) -> None: ...
    def ocr(self, image_path: str, *, use_gpu: bool = False) -> str: ...

# pipelines.ingestion.table_extractor
class TableExtractor:
    def extract(self, image_path: str) -> list[Table]: ...

# pkg.models.document
class ParsedDocument(BaseModel):
    text: str
    tables: list[Table]
    metadata: DocumentMetadata
    status: str  # "ok" | "ocr_fallback" | "parse_failed"
    inference_backend: str | None  # "vllm_gpu" | "transformers_cpu" | None

class Table(BaseModel):
    rows: list[list[str]]
    rotation: int  # 0 | 90 | 180 | 270

class DocumentMetadata(BaseModel):
    author: str | None
    created_date: str | None
    sections: list[str]
```

---

## Synthetic Fixture 전략

실제 KEPCO E&C HWP는 gitignore. CI는 합성 픽스처만 사용.

| 픽스처 | 경로 | 생성 방법 |
|--------|------|----------|
| 정상 HWP (5페이지) | tests/fixtures/synthetic/sample_report.hwp | GREEN phase에서 generate_fixtures.py 생성 |
| 손상 HWP | tests/fixtures/synthetic/corrupted_ole.hwp | GREEN phase에서 생성 |
| 회전 PDF | tests/fixtures/synthetic/rotated_table.pdf | GREEN phase에서 reportlab으로 생성 |

RED phase에서는 unittest.mock으로 파일 I/O를 대체.

---

## Sprint Contract 버전

- v1.0 (2026-05-14): RED phase 초안 작성
