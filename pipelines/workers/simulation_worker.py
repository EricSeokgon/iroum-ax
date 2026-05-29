"""SPEC-AX-001 REQ-AX-003 — Celery 비동기 등급 시뮬레이션 워커

파이프라인: BenchmarkLearner.learn() + GradePredictor.train() + ScenarioSimulator.simulate()
→ post_callback (Go Control Plane)

REQ-INTEG-001 — Celery task name 'pipelines.workers.simulation_worker.run' 보존.
REQ-INTEG-002 — 처리 완료 시 Go Control Plane으로 callback POST.
REQ-INTEG-003 — callback fire-and-forget (예외 흡수, D2).

# @MX:NOTE: [AUTO] simulation_worker — REQ-AX-003 시뮬레이션 Celery 진입점
# @MX:SPEC: SPEC-AX-001 REQ-AX-003
"""
from __future__ import annotations

import logging
from typing import Any

from pkg.models.simulation import BenchmarkReport

from pipelines.callbacks.control_plane import post_callback
from pipelines.config.settings import settings
from pipelines.scoring.benchmark_learner import BenchmarkLearner
from pipelines.scoring.grade_predictor import GradePredictor
from pipelines.scoring.scenario_simulator import ScenarioSimulator

logger = logging.getLogger(__name__)

# @MX:NOTE: [AUTO] TASK_NAME — Kombu envelope headers.task와 정확 일치 필수 (마법 상수)
# @MX:SPEC: SPEC-AX-001 REQ-AX-003 (Go dispatcher도 동일 문자열 사용)
TASK_NAME: str = "pipelines.workers.simulation_worker.run"


def _execute(
    *,
    workflow_id: str,
    document_id: str = "",
    report_text: str = "",
    target_grade: str = "A",
    benchmark_reports: list[dict[str, str]] | None = None,
    **kwargs: Any,  # noqa: ANN401
) -> dict[str, Any]:
    """REQ-AX-003 — B→A 등급 달성 시뮬레이션 파이프라인.

    # @MX:ANCHOR: [AUTO] _execute — simulation_worker 파이프라인 진입점 (fan_in >= 3)
    # @MX:REASON: run task + 단위 테스트 + 통합 테스트에서 호출
    # @MX:SPEC: SPEC-AX-001 REQ-AX-003

    Args:
        workflow_id: 워크플로우 UUID
        document_id: 처리 대상 문서 ID (결과 추적용)
        report_text: 시뮬레이션할 보고서 텍스트 (현재 등급 예측 입력)
        target_grade: 목표 등급 (기본: 'A')
        benchmark_reports: 벤치마크 학습 데이터 [{"text": "...", "grade": "A"|"B"}, ...]
                           None이면 기본 내장 샘플 데이터 사용 (개발/테스트 모드)

    Returns:
        result_json — {document_id, current_grade, target_grade, current_p_a,
                       projected_p_a, content_changes, feasible, spec}
    """
    result_json: dict[str, Any] = {
        "document_id": document_id,
        "spec": "SPEC-AX-001-REQ-AX-003",
    }
    status: str = "completed"

    try:
        # 벤치마크 학습
        learner = BenchmarkLearner()
        reports = _build_benchmark_reports(benchmark_reports)
        learner.learn(reports)

        predictor = GradePredictor()
        predictor.train(learner.reports)

        simulator = ScenarioSimulator()
        simulator.predictor = predictor

        scenario = simulator.simulate(report_text, target_grade=target_grade)

        result_json["current_grade"] = scenario.current_grade
        result_json["target_grade"] = scenario.target_grade
        result_json["current_p_a"] = scenario.current_p_a
        result_json["projected_p_a"] = scenario.projected_p_a
        result_json["content_changes"] = scenario.content_changes
        result_json["feasible"] = scenario.feasible

        logger.info(
            "시뮬레이션 완료 (workflow_id=%s current_grade=%s projected_p_a=%.3f feasible=%s)",
            workflow_id,
            scenario.current_grade,
            scenario.projected_p_a,
            scenario.feasible,
        )

    except Exception as exc:  # noqa: BLE001
        status = "failed"
        result_json["error"] = str(exc)
        logger.error(
            "시뮬레이션 실패 (document_id=%s workflow_id=%s): %s",
            document_id,
            workflow_id,
            exc,
        )

    post_callback(
        base_url=settings.go_control_plane_url,
        workflow_id=workflow_id,
        status=status,
        result_json=result_json,
    )

    logger.info(
        "simulation worker 완료 (document_id=%s workflow_id=%s status=%s)",
        document_id,
        workflow_id,
        status,
    )
    return result_json


def _build_benchmark_reports(
    raw: list[dict[str, str]] | None,
) -> list[BenchmarkReport]:
    """Celery kwargs에서 BenchmarkReport 리스트를 구성한다.

    raw가 None이면 개발·테스트용 최소 샘플 2건을 반환한다.
    (실제 운영 시 Go dispatcher가 학습 데이터를 kwargs로 전달해야 함.)
    """
    if raw:
        return [BenchmarkReport(text_content=r["text"], grade=r["grade"]) for r in raw]

    # 최소 샘플 — 분류기 학습에 A/B 각 1건 이상 필요
    return [
        BenchmarkReport(
            text_content=(
                "안전교육 이수율 100% 달성. 안전사고 0건. "
                "KOSHA 인증 획득. 위험성평가 우수 사례 보유."
            ),
            grade="A",
        ),
        BenchmarkReport(
            text_content="안전교육 이수율 80% 달성. 경상사고 1건 발생.",
            grade="B",
        ),
    ]


try:  # noqa: SIM105 — celery 미설치 환경 분기 명시
    from pipelines.config.celery_client import create_celery_app

    _app = create_celery_app()

    @_app.task(name=TASK_NAME, bind=True, max_retries=3)  # type: ignore[misc]
    def run(self, workflow_id: str, *, document_id: str = "", **kwargs: Any) -> dict[str, Any]:  # noqa: ANN001, ANN401, ARG001
        """Celery task entry — REQ-AX-003 시뮬레이션."""
        return _execute(workflow_id=workflow_id, document_id=document_id, **kwargs)

except ImportError:

    def run(workflow_id: str, *, document_id: str = "", **kwargs: Any) -> dict[str, Any]:  # type: ignore[no-redef]  # noqa: ANN401
        """단위 테스트 격리용 plain function (celery 미설치 시)."""
        return _execute(workflow_id=workflow_id, document_id=document_id, **kwargs)
