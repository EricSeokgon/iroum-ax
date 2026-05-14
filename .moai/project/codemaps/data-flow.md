# E2E 데이터 흐름 (data-flow)

Walking Skeleton 단일 시나리오: HWP 입력 → Recommendation 출력

## 1단계: Document Ingestion (REQ-AX-001)

**입력**: HWP 파일 (예: KEPCO E&C 실적보고서.hwp, 50페이지)

```
[사용자] → POST /api/documents/upload {file}
    ↓
[FastAPI] → validate_file_type() → check 403 if external API
    ↓
[control-plane] → dispatch Celery task (celery.ingestion)
    ↓
[Celery Worker]
    1. hwp_parser.parse_hwp(file_path)
       └─ OLE 구조 파싱 → {text, tables, metadata}
    
    2. language_detector.detect_language(text)
       └─ REQ-UBI-002: 한국어만 진행
    
    3. IF hwp_parser fails:
       └─ vlm_processor.process_ocr(image_path, "Qwen2-VL-7B")
          (vLLM inference, <2sec/page)
       └─ table_extractor.extract_table_structure(vlm_output)
       └─ mark document.status = 'ocr_fallback'
    
    4. PostgreSQL: INSERT documents(...)
       └ id=UUID, filename, file_type, parsed_text, status
```

**출력**: Document 레코드

```json
{
  "id": "uuid-ax-001",
  "filename": "KEPCO_실적보고서.hwp",
  "file_type": "HWP",
  "parsed_text": "안전보건 관련 실적: ... 1200자",
  "tables": [
    {
      "title": "안전교육 이수 현황",
      "rows": [["항목", "2024년"], ["전사원", "98%"]]
    }
  ],
  "status": "parsed",
  "created_at": "2026-05-14T10:00:00Z"
}
```

**Audit Log**:
```sql
INSERT audit_logs (user_id, action, resource_type, resource_id, timestamp)
VALUES ('cli-anonymous', 'UPLOAD', 'document', 'uuid-ax-001', NOW());
```

---

## 2단계: Criterion Mapping & RAG (REQ-AX-002)

**입력**: Document + 평가편람 PDF

```
[사용자] 사전 업로드 (초회 1회만)
    → POST /api/criteria/handbook {평가편람.pdf}
       ↓
       criterion_parser.parse_criterion_handbook(pdf_path)
       └─ Extract: Item(item_id="AX-001", 이름="안전보건", max_points=10)
                   └─ Indicator(name="안전교육 이수율", points=3)
                   └─ Detail(description="...상세 기준...", points=3)
       
       ↓ chunk by 500-1000 tokens
       
       embedding_service.embed_text(chunk, "ko-sroberta-multitask")
       └─ embedding = [0.123, 0.456, ..., 768 dims]
       
       ↓
       
       vector_store.upsert_vectors(embeddings)
       └─ pgvector HNSW index update
       └─ PostgreSQL: INSERT criteria(criterion_name, embedding::vector(768))

---

[자동 RAG 검색]
    
    retriever.retrieve(query="안전교육 이수율", top_k=3)
       ↓
       postgres> SELECT * FROM criteria
                 WHERE embedding <-> {query_embedding} < 0.3
                 ORDER BY embedding <-> {query_embedding}
                 LIMIT 3
       
       ↓ (결과: top-3 + relevance score)
       
       RETURN {
         [
           {"criterion_id": "crit-001", "name": "안전교육 이수율", "relevance": 0.92},
           {"criterion_id": "crit-002", "name": "정기안전 점검", "relevance": 0.85},
           ...
         ]
       }
```

**출력**: RAG Context (최대 3개 평가기준)

```json
{
  "query": "안전교육 이수율",
  "results": [
    {
      "criterion_id": "crit-ax-001",
      "name": "안전교육 이수율",
      "description": "직원의 정기 안전교육 이수율...",
      "max_points": 3,
      "relevance": 0.92
    }
  ]
}
```

**상태**: pgvector 인덱스 READY

---

## 3단계: Grade Simulation (REQ-AX-003)

**입력**: Document (parsed_text) + Benchmark 데이터

```
[사전 준비: Benchmark 학습]
    
    benchmark_learner.fit(benchmark_reports_by_grade)
    └─ A 등급 보고서 3개 → feature extraction
    └─ B 등급 보고서 3개 → feature extraction
    └─ Learn decision boundary (2-class classifier)
    
    ↓
    
[등급 예측]
    
    grade_predictor.predict_probabilities(document.parsed_text)
    
    1. Feature extraction on customer report
    2. 2-class softmax: P(A), P(B)
    3. Abstain check: if max(P(A), P(B)) < 0.5
       └─ abstain = 1.0 - P(A) - P(B)
    4. Return {A: 0.35, B: 0.42, abstain: 0.23}
    
    ↓
    
    scenario_simulator.simulate_scenarios(...)
    └─ Scenario 1: "안전교육 이수율 +5%" → new_score = current + delta
    └─ Scenario 2: "중대재해 투자비 +1000만원" → ...
    └─ Generate 3-5 scenarios
    
    ↓
    
    PostgreSQL: INSERT simulations(...)
       workflow_id, current_grade='B', target_grade='A',
       probabilities={'A': 0.35, 'B': 0.42, 'abstain': 0.23},
       abstain_flag=false, scenarios=[...]
```

**출력**: Simulation 레코드

```json
{
  "id": "sim-001",
  "current_grade": "B",
  "target_grade": "A",
  "probabilities": {
    "A": 0.35,
    "B": 0.42,
    "abstain": 0.23
  },
  "abstain_flag": false,
  "confidence_level": "medium",
  "scenarios": [
    {
      "scenario_name": "안전교육 이수율 +5%",
      "estimated_score_delta": 2,
      "new_estimated_grade": "B+",
      "probability": 0.51
    }
  ]
}
```

---

## 4단계: Report Draft Generation (REQ-AX-004)

**입력**: Document + RAG Context + Simulation 결과

```
[프롬프트 구성]
    
    prompt_builder.build_prompt(
      criterion=RAG_context[0],  # "안전교육 이수율"
      instruction="한국 공공기관 경영평가 기준 준수 보고서 초안 작성",
      examples=BENCHMARK_A_GRADE_EXAMPLES,
      customer_data=document.parsed_text
    )
    
    └─ 생성된 프롬프트 (약 2000 tokens)
    
    ↓
    
[LLM 호출]
    
    llm_client.generate_text(prompt, model="exaone-3.5-7b", max_tokens=512)
    
    try:
      1. Call vLLM endpoint (EXAONE 3.5 7B)
         └─ response: "본 기관의 안전교육은 매년 정기적으로 시행되며..."
      catch:
         └─ Fallback to Qwen 2.5 7B
    
    ↓
    
[스타일 검증]
    
    style_applier.validate_korean_style(draft_text)
    
    └─ Check: 합니다체 준수, 존댓말 없음, 일관된 높임말
    └─ IF violation: re-prompt LLM (retry up to 3x)
    └─ Mark: style_validation_passed = true/false
    
    ↓
    
    PostgreSQL: INSERT reports(...)
       organization_name='KEPCO E&C',
       grade='B',
       content=draft_text,
       generation_model='exaone-3.5-7b',
       style_validation_passed=true
```

**출력**: Report 레코드 (초안)

```json
{
  "id": "rep-001",
  "organization_name": "KEPCO E&C",
  "grade": "B",
  "content": "본 기관의 안전교육은 매년 정기적으로 시행되며...",
  "sections": [
    {
      "criterion_id": "crit-ax-001",
      "section_title": "안전교육 이수율",
      "content": "...상세 내용...",
      "confidence_score": 0.87
    }
  ],
  "generation_model": "exaone-3.5-7b",
  "style_validation_passed": true
}
```

---

## 5단계: Gap Recommendation (REQ-AX-005)

**입력**: Document + Report (초안) + Simulation (A등급 시나리오)

```
[Gap 분석]
    
    gap_analyzer.analyze_gap(
      current_report=report_record,
      target_grade='A',
      benchmark_content_index=RAG_BENCHMARK_A_INDEX
    )
    
    └─ Compare keyword frequencies: current vs. benchmark_A
    └─ Identify missing sections
    └─ Extract gap items
    
    ↓
    
[콘텐츠 제안]
    
    content_suggester.suggest_content(gap_analysis, benchmark_index)
    
    └─ For each gap:
       ├─ Search benchmark_A reports for matching content
       ├─ Extract recommendation text
       └─ Store as Recommendation object
    
    ↓
    
[우선순위 정렬]
    
    prioritizer.rank_by_feasibility(suggestions)
    
    └─ Feasibility score = (implementation_effort inverse) × (impact score)
    └─ Priority 1 = highest feasibility
    
    ↓
    
    PostgreSQL: INSERT recommendations(...)
       FOR EACH recommendation IN recommendations_list:
          simulation_id, criterion_id, item_title, 
          current_value, recommended_value, 
          estimated_score_delta, feasibility_score, priority
```

**출력**: Recommendation 배치

```json
{
  "simulation_id": "sim-001",
  "items": [
    {
      "id": "rec-001",
      "criterion_id": "crit-ax-001",
      "item_title": "안전교육 이수율 → 목표 100% 상향",
      "current_value": "98%",
      "recommended_value": "100%",
      "estimated_score_delta": 2,
      "feasibility_score": 0.92,
      "priority": 1,
      "reason": "A 등급 벤치마크 기관들이 100% 달성",
      "reference_sources": ["rep-bench-a-001", "rep-bench-a-002"]
    },
    {
      "id": "rec-002",
      ...
    }
  ],
  "total_score_improvement": 5
}
```

---

## E2E 데이터 체인 요약

```
HWP Document
    ↓ (파싱 + OCR)
    ↓
Structured Document (text + tables)
    ↓ (RAG 검색)
    ↓
Criterion Matches (top-3)
    ↓ (특징 추출)
    ↓
Grade Probabilities {A: %, B: %, abstain: %}
    ↓ (LLM 생성)
    ↓
Draft Report (합니다체 검증)
    ↓ (벤치마크 비교)
    ↓
Recommendations (우선순위 정렬)
    ↓
[최종 사용자 응답]
```

**전체 처리 시간**: ~10-15초 (sequen tial, GPU 활성화 기준)

---

**최종 업데이트**: 2026-05-14 (Sprint 6 완료)  
**시나리오**: KEPCO E&C 안전보건 항목 1개, 평가 1회  
**PostgreSQL 테이블**: documents·criteria·reports·simulations·recommendations·audit_logs
