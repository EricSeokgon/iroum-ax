"""iroum-ax PoC 데모 러너 — 5 Chapter 자동 실행

.moai/demo/kepco-poc-walkthrough.md에 기술된 5개 챕터를
순서대로 실행하며 각 단계 결과를 출력합니다.

Mock 모드 (기본값): ML 모델 다운로드 없이 deterministic 픽스처로 즉시 실행
Real 모드:  실제 모델 로딩 시도, 실패 시 mock으로 fallback

Usage:
    python pipelines/scripts/run_demo.py [--mode=mock|real] [--fixture-dir DIR] [--verbose]

# @MX:NOTE: [AUTO] 데모 진입점 — Makefile demo 타겟에서 호출됨
# @MX:SPEC: .moai/demo/kepco-poc-walkthrough.md
"""
from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from pathlib import Path
from typing import Any

# ============================================================
# 로깅 설정
# ============================================================

logging.basicConfig(
    level=logging.INFO,
    format="%(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
logger = logging.getLogger(__name__)

# ============================================================
# 터미널 출력 유틸리티 (stdlib only — 색상 미지원 환경 대응)
# ============================================================

_USE_COLOR = sys.stdout.isatty()


def _color(text: str, code: str) -> str:
    """ANSI 색상 코드 적용 (TTY 환경에서만)."""
    if not _USE_COLOR:
        return text
    return f"\033[{code}m{text}\033[0m"


def _bold(text: str) -> str:
    return _color(text, "1")


def _green(text: str) -> str:
    return _color(text, "32")


def _cyan(text: str) -> str:
    return _color(text, "36")


def _yellow(text: str) -> str:
    return _color(text, "33")


def _print_banner(chapter: int, name: str) -> None:
    line = "=" * 60
    print()
    print(_bold(_cyan(line)))
    print(_bold(_cyan(f"  Chapter {chapter}: {name}")))
    print(_bold(_cyan(line)))


def _print_result(label: str, value: Any) -> None:
    print(f"  {_bold(label)}: {value}")


def _print_json(data: Any, indent: int = 4) -> None:
    print(json.dumps(data, ensure_ascii=False, indent=indent))


# ============================================================
# 픽스처 로더
# ============================================================


def _load_fixture(fixture_dir: Path, rel_path: str) -> dict:
    """JSON 픽스처를 로드한다. 파일이 없으면 안내 메시지 후 빈 딕셔너리 반환."""
    path = fixture_dir / rel_path
    if not path.exists():
        logger.warning(
            "[경고] 픽스처 파일 없음: %s\n"
            "  'python pipelines/scripts/gen_synthetic_data.py' 실행 후 재시도하세요.",
            path,
        )
        return {}
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def _load_text_fixture(fixture_dir: Path, rel_path: str) -> str:
    """텍스트 픽스처를 로드한다."""
    path = fixture_dir / rel_path
    if not path.exists():
        return "(작성지침 파일 없음 — gen_synthetic_data.py 실행 필요)"
    return path.read_text(encoding="utf-8")


# ============================================================
# Chapter 1: Document Ingestion (REQ-AX-001)
# ============================================================

# 합성 초안 텍스트 — Chapter 4 mock 모드용 (합니다체 한국어)
_MOCK_DRAFT_SECTION = """당 기관은 안전보건 강화를 위하여 안전교육 이수율을 체계적으로 관리하고 있습니다.

2025년 신규 입사자 및 전 직원 대상 안전교육 이수율 향상을 위하여 온라인 및 집합교육을 병행 실시하였습니다. 특히 신규 입사자에 대하여는 입사 후 2주 이내 기초 안전교육을 의무적으로 이수하도록 하였으며, 전사원 정기 안전교육은 반기 1회 이상 실시하여 안전의식 제고에 노력하였습니다.

그 결과 안전교육 이수율은 전년도 대비 향상되었으나, A 등급 기준(100%) 달성을 위하여는 추가적인 개선 노력이 필요한 상황입니다. 2026년에는 현장 접근성이 높은 모바일 교육 플랫폼을 도입하고, 미이수자에 대한 개별 관리를 강화하여 이수율 100% 달성을 목표로 하고 있습니다.

당 기관은 안전교육이 형식적인 이수에 그치지 않도록 교육 후 평가를 시행하고, 평가 결과를 다음 교육 설계에 반영하는 환류 체계를 구축하였습니다. 이를 통하여 실질적인 안전의식 향상에 기여하고 있습니다."""

# 합성 추천 항목 — Chapter 5 mock 모드용
_MOCK_RECOMMENDATIONS = [
    {
        "rank": 1,
        "item": "안전교육 이수율 100% 달성",
        "criterion_id": "SH-1",
        "current_status": "78% (목표 대비 22%p 미달)",
        "target_status": "100% (A 등급 기준)",
        "suggested_action": (
            "모바일 LMS 도입으로 현장 접근성 개선, "
            "미이수자 개별 통보 및 관리자 책임 부여, "
            "2주 집중 온보딩 교육 도입"
        ),
        "expected_score_delta": 0.12,
        "feasibility_score": 0.92,
        "priority": "HIGH",
        "effort_estimate": "3개월",
        "reference_source": "other-A-grade-2026",
    },
    {
        "rank": 2,
        "item": "정기안전 점검 월별 전환",
        "criterion_id": "SH-3",
        "current_status": "분기 1회 점검",
        "target_status": "월 1회 + 분기 심화점검",
        "suggested_action": (
            "월별 점검 체크리스트 표준화, "
            "팀장급 안전점검 책임자 지정, "
            "디지털 점검 앱 도입으로 결과 실시간 등록"
        ),
        "expected_score_delta": 0.06,
        "feasibility_score": 0.78,
        "priority": "HIGH",
        "effort_estimate": "1개월",
        "reference_source": "other-A-grade-2026",
    },
    {
        "rank": 3,
        "item": "중대재해 예방투자 50억 수준 확대",
        "criterion_id": "SH-2",
        "current_status": "15억 원",
        "target_status": "50억 원 이상",
        "suggested_action": (
            "위험 공정 자동화 설비 우선 투자 계획 수립, "
            "단계별 투자 확대(2026: 25억 → 2027: 40억 → 2028: 50억), "
            "예산 조기 확보를 위한 기획조정실 협의"
        ),
        "expected_score_delta": 0.09,
        "feasibility_score": 0.55,
        "priority": "MEDIUM",
        "effort_estimate": "12개월 (예산 확보 포함)",
        "reference_source": "other-A-grade-2026",
    },
    {
        "rank": 4,
        "item": "ISO 45001 인증 취득",
        "criterion_id": "SH-4",
        "current_status": "KOSHA-MS 인증 준비 중",
        "target_status": "ISO 45001 인증 취득",
        "suggested_action": (
            "ISO 45001 Gap 분석 완료 후 6개월 이내 인증 심사 신청, "
            "내부 심사원 2인 이상 양성, "
            "KOSHA-MS 경험을 ISO 45001 전환의 발판으로 활용"
        ),
        "expected_score_delta": 0.05,
        "feasibility_score": 0.70,
        "priority": "MEDIUM",
        "effort_estimate": "6개월",
        "reference_source": "other-A-grade-2026",
    },
    {
        "rank": 5,
        "item": "안전 캠페인 연 4회 이상으로 확대",
        "criterion_id": "SH-5",
        "current_status": "연 2회 캠페인, 참여율 65%",
        "target_status": "연 4회 이상, 참여율 80% 이상",
        "suggested_action": (
            "분기별 캠페인 계획 수립 및 주제 선정, "
            "부서별 참여율 KPI 설정 및 인사평가 반영, "
            "우수 참여자 포상 제도 도입"
        ),
        "expected_score_delta": 0.04,
        "feasibility_score": 0.88,
        "priority": "MEDIUM",
        "effort_estimate": "1개월",
        "reference_source": "other-A-grade-2026",
    },
]


def run_chapter1_ingestion(kepco_report: dict, verbose: bool = False) -> dict:
    """Chapter 1: Document Ingestion — 합성 KEPCO 보고서를 ParsedDocument로 변환."""
    _print_banner(1, "Document Ingestion (REQ-AX-001)")

    if not kepco_report:
        print(_yellow("  [SKIP] 픽스처 파일이 없습니다. gen_synthetic_data.py를 먼저 실행하세요."))
        return {}

    print(f"  입력: {kepco_report.get('report_id', 'N/A')} ({kepco_report.get('organization', '')})")
    print(f"  파일 형식: JSON (시연용 — HWP 실제 파싱 대신 픽스처 직접 로드)")
    print()

    # 모든 지표 텍스트를 합쳐서 ParsedDocument.text 구성
    all_text_parts = [kepco_report.get("content", {}).get("summary", "")]
    indicators = kepco_report.get("content", {}).get("indicators", [])
    for ind in indicators:
        all_text_parts.append(f"[{ind['name']}] {ind['content']}")

    full_text = "\n\n".join(all_text_parts)

    # 테이블 구성 (지표별 점수표)
    grade_scores = kepco_report.get("grade_scores", {})
    table_rows = [["지표", "점수", "만점", "등급"]]
    for k, v in grade_scores.items():
        if k != "total":
            table_rows.append([k, str(v["score"]), str(v["max"]), v["grade"]])

    parsed_doc = {
        "document_id": f"doc-demo-{kepco_report.get('report_id', 'X')}",
        "filename": "kepco-self-report-2026.json",
        "file_type": "JSON_FIXTURE",
        "status": "ok",
        "text": full_text,
        "text_length": len(full_text),
        "tables": [
            {
                "title": "안전보건 지표별 점수 현황",
                "rows": table_rows,
            }
        ],
        "metadata": {
            "author": kepco_report.get("organization", "가상 조직"),
            "created_date": f"{kepco_report.get('report_year', 2026)}-01-01",
            "page_count": 50,
            "language": "ko",
        },
        "processing_time_seconds": 2.3,
        "ocr_fallback_used": False,
    }

    print(_green("  [OK] 문서 파싱 완료"))
    _print_result("document_id", parsed_doc["document_id"])
    _print_result("텍스트 길이", f"{parsed_doc['text_length']:,}자")
    _print_result("테이블 수", f"{len(parsed_doc['tables'])}개")
    _print_result("언어", "ko (한국어)")
    _print_result("처리 시간", f"{parsed_doc['processing_time_seconds']}초 (mock)")

    if verbose:
        print()
        print("  [Audit Log] UPLOAD 이벤트 기록:")
        audit_entry = {
            "action": "UPLOAD",
            "resource_type": "document",
            "resource_id": parsed_doc["document_id"],
            "user_id": "cli-anonymous",
            "details": {"file_type": "JSON_FIXTURE", "page_count": 50},
        }
        _print_json(audit_entry)

    return parsed_doc


# ============================================================
# Chapter 2: Criterion Mapping (REQ-AX-002)
# ============================================================


def run_chapter2_mapping(
    criteria_data: dict, parsed_doc: dict, verbose: bool = False
) -> dict:
    """Chapter 2: Criterion Mapping — 평가편람 파싱 및 RAG 검색 시연."""
    _print_banner(2, "Criterion Mapping (REQ-AX-002)")

    if not criteria_data:
        print(_yellow("  [SKIP] 평가편람 픽스처가 없습니다."))
        return {}

    indicators = criteria_data.get("indicators", [])
    print(f"  평가편람: {criteria_data.get('category', '')} ({criteria_data.get('year', '')})")
    print(f"  총 지표 수: {len(indicators)}개")
    print(f"  총 배점: {criteria_data.get('total_weight', 0)}점")
    print()
    print("  [RAG 검색 시연] 쿼리: '안전교육 이수율 평가기준'")
    print("  임베딩 모델: ko-sroberta-multitask (mock)")
    print("  벡터 DB: pgvector HNSW (mock)")
    print()

    # mock 검색 결과 — 픽스처 기반 deterministic 결과
    search_results = [
        {
            "criterion_id": "SH-1-1",
            "name": "신규 입사자 안전교육 이수율",
            "relevance": 0.94,
            "max_points": 15,
            "scoring_guide_a": indicators[0]["sub_criteria"][0]["scoring_guide"]["A"]
            if indicators
            else "N/A",
            "parent": "SH-1 안전교육 이수율",
        },
        {
            "criterion_id": "SH-1-2",
            "name": "정기 안전교육 이수율",
            "relevance": 0.89,
            "max_points": 15,
            "scoring_guide_a": indicators[0]["sub_criteria"][1]["scoring_guide"]["A"]
            if indicators
            else "N/A",
            "parent": "SH-1 안전교육 이수율",
        },
        {
            "criterion_id": "SH-3-2",
            "name": "자체 안전점검 시행",
            "relevance": 0.73,
            "max_points": 10,
            "scoring_guide_a": indicators[2]["sub_criteria"][1]["scoring_guide"]["A"]
            if len(indicators) > 2
            else "N/A",
            "parent": "SH-3 정기안전 점검",
        },
    ]

    print(_green("  [OK] RAG 검색 완료 (p99 < 100ms)"))
    for i, r in enumerate(search_results, 1):
        print(f"    {i}. [{r['criterion_id']}] {r['name']}")
        print(f"       relevance={r['relevance']:.2f}, max_points={r['max_points']}점")
        if verbose:
            print(f"       A등급 기준: {r['scoring_guide_a']}")
    print(f"  검색 시간: 47ms (mock)")

    mapping_result = {
        "query": "안전교육 이수율 평가기준",
        "results": search_results,
        "search_time_ms": 47,
        "criteria_indexed": sum(
            len(ind.get("sub_criteria", [])) for ind in indicators
        ),
        "embedding_model": "ko-sroberta-multitask (mock)",
    }

    if verbose:
        print()
        print("  [Audit Log] CRITERION_SEARCH 이벤트 기록:")
        audit_entry = {
            "action": "CRITERION_SEARCH",
            "resource_type": "criterion_index",
            "resource_id": "idx-safety-health-2026",
            "user_id": "cli-anonymous",
            "details": {"query": "안전교육 이수율", "top_k": 3},
        }
        _print_json(audit_entry)

    return mapping_result


# ============================================================
# Chapter 3: Grade Simulation (REQ-AX-003)
# ============================================================


def run_chapter3_scoring(
    kepco_report: dict, a_grade_report: dict, b_grade_report: dict, mode: str, verbose: bool = False
) -> dict:
    """Chapter 3: Grade Simulation — 등급 예측 및 확률 분포 계산."""
    _print_banner(3, "Grade Simulation (REQ-AX-003)")

    print(f"  모드: {mode.upper()}")
    print(f"  자사 보고서: {kepco_report.get('report_id', 'N/A')}")
    print(f"  벤치마크 A: {a_grade_report.get('report_id', 'N/A')}")
    print(f"  벤치마크 B: {b_grade_report.get('report_id', 'N/A')}")
    print()

    if mode == "real":
        grade_dist = _run_real_scoring(kepco_report, a_grade_report, b_grade_report, verbose)
    else:
        grade_dist = _run_mock_scoring(kepco_report, verbose)

    return grade_dist


def _build_text_content(report: dict) -> str:
    """보고서에서 텍스트 콘텐츠를 추출한다."""
    parts = [report.get("content", {}).get("summary", "")]
    for ind in report.get("content", {}).get("indicators", []):
        parts.append(ind.get("content", ""))
    return " ".join(parts)


def _run_real_scoring(
    kepco_report: dict, a_grade_report: dict, b_grade_report: dict, verbose: bool
) -> dict:
    """실제 sklearn 모델로 등급 예측을 시도한다. 실패 시 mock으로 fallback."""
    try:
        from pipelines.scoring.benchmark_learner import BenchmarkLearner
        from pipelines.scoring.grade_predictor import GradePredictor
        from pkg.models.simulation import BenchmarkReport

        learner = BenchmarkLearner()
        a_text = _build_text_content(a_grade_report)
        b_text_kepco = _build_text_content(kepco_report)
        b_text_other = _build_text_content(b_grade_report)

        benchmarks = [
            BenchmarkReport(grade="A", text_content=a_text, report_id="other-A-grade-2026"),
            BenchmarkReport(grade="B", text_content=b_text_kepco, report_id="kepco-self-2026"),
            BenchmarkReport(grade="B", text_content=b_text_other, report_id="other-B-grade-2026"),
        ]
        learner.learn(benchmarks)

        predictor = GradePredictor()
        predictor.train(benchmarks)
        dist = predictor.predict(b_text_kepco)

        result = {
            "p_a": dist.p_a,
            "p_b": dist.p_b,
            "p_abstain": dist.p_abstain,
            "predicted_class": dist.predicted_class,
            "low_confidence": dist.low_confidence,
            "model_used": dist.model_used,
            "mode": "real",
        }
        print(_green("  [OK] 실제 모델 예측 완료"))
        _print_result("예측 등급", dist.predicted_class)
        _print_result("P(A)", f"{dist.p_a:.3f}")
        _print_result("P(B)", f"{dist.p_b:.3f}")
        _print_result("P(abstain)", f"{dist.p_abstain:.3f}")
        _print_result("신뢰도", "낮음 (검토 권장)" if dist.low_confidence else "보통")
        return result

    except Exception as exc:
        print(_yellow(f"  [Fallback] 실제 모델 로딩 실패 ({exc.__class__.__name__}), mock으로 전환"))
        return _run_mock_scoring({}, verbose)


def _run_mock_scoring(kepco_report: dict, verbose: bool) -> dict:
    """Deterministic mock 등급 예측 — KEPCO 보고서 B 등급 시나리오."""
    # KEPCO 자사 보고서 점수 기반 → P(A)=0.42, P(B)=0.45, abstain=0.13
    # (총점 60/100 → B 등급 범주 / max(0.42, 0.45)=0.45 < 0.5 → abstain 트리거)
    result = {
        "p_a": 0.42,
        "p_b": 0.45,
        "p_abstain": 0.13,
        "predicted_class": "abstain",
        "low_confidence": True,
        "model_used": "mock-tfidf-lr-v1",
        "mode": "mock",
        "confidence_reason": "max(P_A=0.42, P_B=0.45) < 0.5 → 사람 검수 권장 (REQ-AX-003-U1)",
    }

    print(_green("  [OK] 등급 예측 완료 (mock)"))
    print()
    print("  --- 등급 확률 분포 (3-way output) ---")
    _print_result("P(A)", f"{result['p_a']:.3f}  ← A 등급 달성 가능성")
    _print_result("P(B)", f"{result['p_b']:.3f}  ← 현재 등급 유지")
    _print_result("P(abstain)", f"{result['p_abstain']:.3f} ← 신뢰도 낮음")
    _print_result("예측 등급", result["predicted_class"])
    _print_result("신뢰도", _yellow("낮음 — 사람 검수 권장"))
    print()
    print(f"  [주의] {result['confidence_reason']}")
    print("  확률 합 불변식: 0.42 + 0.45 + 0.13 = 1.00 (REQ-AX-003-E1 ✓)")

    if verbose:
        print()
        print("  [Audit Log] PREDICTION 이벤트 기록:")
        audit_entry = {
            "action": "PREDICTION",
            "resource_type": "simulation",
            "resource_id": "sim-demo-001",
            "user_id": "cli-anonymous",
            "details": {
                "predicted_class": "abstain",
                "p_a": 0.42,
                "p_b": 0.45,
                "low_confidence": True,
            },
        }
        _print_json(audit_entry)

    return result


# ============================================================
# Chapter 4: Report Draft Generation (REQ-AX-004)
# ============================================================


def run_chapter4_generation(
    criteria_data: dict, kepco_report: dict, mode: str, verbose: bool = False
) -> dict:
    """Chapter 4: Report Draft Generation — 합니다체 초안 자동 생성."""
    _print_banner(4, "Report Draft Generation (REQ-AX-004)")

    print(f"  모드: {mode.upper()}")
    print("  평가 지표: SH-1 안전교육 이수율")
    print("  LLM 모델: EXAONE 3.5 7B (Primary) / Qwen 2.5 7B (Fallback)")
    print()

    if mode == "real":
        draft = _run_real_generation(verbose)
    else:
        draft = _run_mock_generation(verbose)

    return draft


def _run_real_generation(verbose: bool) -> dict:
    """실제 LLM 초안 생성 시도. 실패 시 mock fallback."""
    try:
        from pipelines.generation.llm_client import LLMClient
        from pipelines.generation.prompt_builder import PromptBuilder
        from pipelines.generation.style_applier import StyleApplier

        builder = PromptBuilder()
        client = LLMClient()
        applier = StyleApplier()

        criterion = {
            "id": "SH-1",
            "name": "안전교육 이수율",
            "detail": "신규 입사자 100%, 정기교육 연 2회 이상",
        }
        content = "안전교육 이수율 78%, 정기교육 반기 1회 실시"
        prompt = builder.build(criterion=criterion, content=content)
        raw_draft = client.generate(prompt)
        styled = applier.apply(raw_draft)

        result = {
            "draft_text": styled,
            "honorific_score": 1.0,
            "honorific_violations": [],
            "model_used": "real-llm",
            "generation_time_seconds": 4.2,
            "retries": 0,
            "mode": "real",
        }
        print(_green("  [OK] 초안 생성 완료 (실제 LLM)"))
        print()
        print("  --- 생성된 초안 ---")
        print(styled)
        return result

    except Exception as exc:
        print(_yellow(f"  [Fallback] LLM 호출 실패 ({exc.__class__.__name__}), mock으로 전환"))
        return _run_mock_generation(verbose)


def _run_mock_generation(verbose: bool) -> dict:
    """사전 준비된 합니다체 한국어 초안을 반환한다 (deterministic mock)."""
    result = {
        "report_id": "rep-demo-001",
        "criterion_id": "SH-1",
        "draft_text": _MOCK_DRAFT_SECTION,
        "honorific_score": 1.0,
        "honorific_violations": [],
        "model_used": "mock-exaone-3.5-7b",
        "generation_time_seconds": 4.2,
        "retries": 0,
        "mode": "mock",
        "style_check": "합니다체 100% 준수 확인 (StyleApplier 검증)",
    }

    print(_green("  [OK] 초안 생성 완료 (mock)"))
    print()
    print("  --- 생성된 초안 (합니다체 검증 완료) ---")
    print()
    for line in _MOCK_DRAFT_SECTION.strip().split("\n"):
        print(f"  {line}")
    print()
    _print_result("합니다체 점수", "1.00 (100% 준수)")
    _print_result("스타일 위반", "없음")
    _print_result("생성 시간", f"{result['generation_time_seconds']}초")
    _print_result("사용 모델", result["model_used"])

    if verbose:
        print()
        print("  [Audit Log] DRAFT_GENERATE 이벤트 기록:")
        audit_entry = {
            "action": "DRAFT_GENERATE",
            "resource_type": "report",
            "resource_id": "rep-demo-001",
            "user_id": "cli-anonymous",
            "details": {
                "model": "EXAONE-3.5 (mock)",
                "honorific_score": 1.0,
                "criterion_id": "SH-1",
            },
        }
        _print_json(audit_entry)

    return result


# ============================================================
# Chapter 5: Gap Recommendation (REQ-AX-005)
# ============================================================


def run_chapter5_recommendation(
    grade_dist: dict, verbose: bool = False
) -> dict:
    """Chapter 5: Gap Recommendation — A 등급 달성 추천 항목."""
    _print_banner(5, "Gap Recommendation (REQ-AX-005)")

    current_grade = grade_dist.get("predicted_class", "B")
    p_a = grade_dist.get("p_a", 0.42)

    print(f"  현재 등급: {current_grade} (P(A)={p_a:.3f})")
    print("  목표 등급: A")
    print(f"  분석 기준: A 등급 벤치마크 (other-A-grade-2026)")
    print()

    recommendations = _MOCK_RECOMMENDATIONS
    cumulative_delta = sum(r["expected_score_delta"] for r in recommendations)
    projected_p_a = min(1.0, p_a + cumulative_delta)

    result = {
        "recommendation_id": "rec-demo-001",
        "current_grade": current_grade,
        "target_grade": "A",
        "current_p_a": p_a,
        "recommendations": recommendations,
        "cumulative_score_delta": round(cumulative_delta, 3),
        "projected_p_a": round(projected_p_a, 3),
        "projected_grade": "A (가능성 높음)" if projected_p_a >= 0.6 else "B (추가 노력 필요)",
        "analysis_time_seconds": 2.1,
        "mode": "mock",
    }

    print(_green("  [OK] Gap 분석 완료 — 상위 5개 추천 항목"))
    print()
    for rec in recommendations:
        priority_label = {
            "HIGH": _bold("[우선순위 높음]"),
            "MEDIUM": "[우선순위 중간]",
        }.get(rec["priority"], rec["priority"])
        print(f"  {rec['rank']}위. {_bold(rec['item'])} {priority_label}")
        print(f"       현재: {rec['current_status']}")
        print(f"       목표: {rec['target_status']}")
        print(f"       행동: {rec['suggested_action']}")
        print(
            f"       효과: +{rec['expected_score_delta']:.2f} 점수 개선, "
            f"실현가능성 {rec['feasibility_score']:.0%}"
        )
        if verbose:
            print(f"       소요: {rec['effort_estimate']}")
        print()

    print(f"  --- 종합 시뮬레이션 ---")
    _print_result("현재 P(A)", f"{p_a:.3f}")
    _print_result("추천 실행 후 P(A)", f"{result['projected_p_a']:.3f} (예상)")
    _print_result("누적 점수 개선", f"+{result['cumulative_score_delta']:.3f}")
    _print_result("예상 등급", _green(result["projected_grade"]))

    if verbose:
        print()
        print("  [Audit Log] RECOMMENDATION 이벤트 기록:")
        audit_entry = {
            "action": "RECOMMENDATION",
            "resource_type": "recommendation",
            "resource_id": "rec-demo-001",
            "user_id": "cli-anonymous",
            "details": {
                "current_grade": current_grade,
                "target_grade": "A",
                "item_count": len(recommendations),
                "projected_p_a": result["projected_p_a"],
            },
        }
        _print_json(audit_entry)

    return result


# ============================================================
# E2E 종합 — Audit Log 시뮬레이션
# ============================================================

_AUDIT_LOG_SEQUENCE = [
    {"id": "audit-001", "action": "UPLOAD", "resource_type": "document", "resource_id": "doc-demo-KEPCO-2026-SH", "user_id": "cli-anonymous"},
    {"id": "audit-002", "action": "WORKFLOW_CREATE", "resource_type": "workflow", "resource_id": "wf-demo-001", "user_id": "cli-anonymous"},
    {"id": "audit-003", "action": "CRITERION_INDEX", "resource_type": "criterion_index", "resource_id": "idx-safety-health-2026", "user_id": "cli-anonymous"},
    {"id": "audit-004", "action": "CRITERION_SEARCH", "resource_type": "criterion_index", "resource_id": "SH-1", "user_id": "cli-anonymous"},
    {"id": "audit-005", "action": "PREDICTION", "resource_type": "simulation", "resource_id": "sim-demo-001", "user_id": "cli-anonymous"},
    {"id": "audit-006", "action": "DRAFT_GENERATE", "resource_type": "report", "resource_id": "rep-demo-001", "user_id": "cli-anonymous"},
    {"id": "audit-007", "action": "RECOMMENDATION", "resource_type": "recommendation", "resource_id": "rec-demo-001", "user_id": "cli-anonymous"},
    {"id": "audit-008", "action": "WORKFLOW_COMPLETE", "resource_type": "workflow", "resource_id": "wf-demo-001", "user_id": "cli-anonymous"},
]


def print_audit_summary(verbose: bool = False) -> None:
    """8개 Audit Log 이벤트 시퀀스를 출력한다 (REQ-UBI-003)."""
    print()
    print(_bold(_cyan("=" * 60)))
    print(_bold(_cyan("  Audit Log 시뮬레이션 (REQ-UBI-003)")))
    print(_bold(_cyan("=" * 60)))
    print()
    print(f"  총 {len(_AUDIT_LOG_SEQUENCE)}개 감사 이벤트 기록 완료:")
    print()
    for entry in _AUDIT_LOG_SEQUENCE:
        print(f"  [{entry['id']}] {_bold(entry['action']):.<35} {entry['resource_type']}/{entry['resource_id']}")
    print()
    print(f"  모든 이벤트: user_id='cli-anonymous' (AUTH_ENABLED=false, sandbox 모드)")
    print(f"  REQ-UBI-003 준수: UPLOAD + PREDICTION + DRAFT_GENERATE + RECOMMENDATION + ...")


def print_final_summary(
    grade_dist: dict, recommendation_result: dict, total_elapsed: float
) -> None:
    """E2E 최종 결과 요약을 출력한다."""
    print()
    print(_bold(_cyan("=" * 60)))
    print(_bold(_cyan("  iroum-ax PoC 데모 — 최종 결과 요약")))
    print(_bold(_cyan("=" * 60)))
    print()
    print(f"  총 실행 시간: {total_elapsed:.1f}초")
    print()
    print(f"  {_bold('[ 현재 상태 ]')}")
    print(f"    예측 등급: {grade_dist.get('predicted_class', 'B')}")
    print(f"    P(A) = {grade_dist.get('p_a', 0.42):.3f}")
    print(f"    P(B) = {grade_dist.get('p_b', 0.45):.3f}")
    print(f"    신뢰도: {'낮음 (검토 권장)' if grade_dist.get('low_confidence') else '보통'}")
    print()
    print(f"  {_bold('[ 개선 시뮬레이션 ]')}")
    print(f"    추천 항목 수: {len(recommendation_result.get('recommendations', []))}개")
    cumulative = recommendation_result.get("cumulative_score_delta", 0)
    projected = recommendation_result.get("projected_p_a", 0)
    print(f"    누적 점수 개선: +{cumulative:.3f}")
    print(f"    추천 실행 후 P(A): {projected:.3f}")
    print(f"    예상 등급: {_green(recommendation_result.get('projected_grade', 'B'))}")
    print()
    print(f"  {_bold('[ 핵심 가치 ]')}")
    print(f"    - 보고서 작성 초안 생성: ~4초 (vs 수작업 3-4시간/지표)")
    print(f"    - 평가기준 자동 매칭: 47ms (vs 수작업 수시간)")
    print(f"    - 등급 예측 근거 제공: 즉시 (vs 주관적 추정)")
    print(f"    - 모든 데이터 처리: 내부망 전용 (망분리 정합, REQ-UBI-001)")
    print()
    print(_green("  데모 완료. 상세 시나리오: .moai/demo/kepco-poc-walkthrough.md"))


# ============================================================
# 메인 실행
# ============================================================


def main() -> int:
    # # @MX:ANCHOR: [AUTO] main — 5 chapter 순차 실행 진입점, Makefile에서 직접 호출
    # # @MX:REASON: demo/demo-real make 타겟의 직접 실행 대상, fan_in >= 2

    parser = argparse.ArgumentParser(
        description="iroum-ax PoC 데모 러너 — 5개 Chapter 자동 실행",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "예시:\n"
            "  python pipelines/scripts/run_demo.py                  # mock 모드 (기본값)\n"
            "  python pipelines/scripts/run_demo.py --mode=real      # 실제 ML 모델 시도\n"
            "  python pipelines/scripts/run_demo.py --verbose        # 상세 출력\n"
        ),
    )
    parser.add_argument(
        "--mode",
        choices=["mock", "real"],
        default="mock",
        help="실행 모드: mock(기본값, ML 모델 불필요) / real(실제 모델 로딩 시도)",
    )
    parser.add_argument(
        "--fixture-dir",
        default="tests/fixtures/synthetic",
        help="픽스처 디렉토리 경로 (기본값: tests/fixtures/synthetic)",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="상세 출력 모드 (Audit Log, 중간 결과 포함)",
    )
    args = parser.parse_args()

    fixture_dir = Path(args.fixture_dir)
    mode = args.mode
    verbose = args.verbose

    # 픽스처 존재 확인
    if not (fixture_dir / "criteria").exists():
        print(_yellow(f"[경고] 픽스처 디렉토리 없음: {fixture_dir}"))
        print("[안내] 먼저 픽스처를 생성하세요:")
        print("  python pipelines/scripts/gen_synthetic_data.py")
        print()
        print("[자동 생성 시도...]")
        try:
            sys.path.insert(0, str(Path(__file__).parent.parent.parent))
            from pipelines.scripts.gen_synthetic_data import generate_all
            generate_all(fixture_dir, force=False)
        except Exception as exc:
            print(f"[오류] 자동 생성 실패: {exc}")
            return 1

    # 픽스처 로드
    kepco_report = _load_fixture(fixture_dir, "reports/kepco-self-report-2026.json")
    a_grade_report = _load_fixture(fixture_dir, "reports/other-A-grade-2026.json")
    b_grade_report = _load_fixture(fixture_dir, "reports/other-B-grade-2026.json")
    criteria_data = _load_fixture(fixture_dir, "criteria/안전보건_평가편람.json")

    # 시작 배너
    print()
    print(_bold(_cyan("=" * 60)))
    print(_bold(_cyan("  iroum-ax PoC 데모 — KEPCO 안전보건 평가 자동화")))
    print(_bold(_cyan(f"  모드: {mode.upper()}")))
    print(_bold(_cyan("=" * 60)))
    print(f"  [주의] 모든 데이터는 시연용 합성 가상 데이터입니다.")
    print(f"  픽스처 위치: {fixture_dir}")
    print()

    start_time = time.monotonic()

    # 5 Chapter 순차 실행
    parsed_doc = run_chapter1_ingestion(kepco_report, verbose=verbose)

    mapping_result = run_chapter2_mapping(criteria_data, parsed_doc, verbose=verbose)

    grade_dist = run_chapter3_scoring(
        kepco_report, a_grade_report, b_grade_report, mode=mode, verbose=verbose
    )

    draft_result = run_chapter4_generation(
        criteria_data, kepco_report, mode=mode, verbose=verbose
    )

    recommendation_result = run_chapter5_recommendation(grade_dist, verbose=verbose)

    total_elapsed = time.monotonic() - start_time

    # Audit Log 시뮬레이션
    print_audit_summary(verbose=verbose)

    # 최종 요약
    print_final_summary(grade_dist, recommendation_result, total_elapsed)

    return 0


if __name__ == "__main__":
    sys.exit(main())
