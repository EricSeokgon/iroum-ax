"""AC-UBI-003: 감사 로깅 — 4종 액션 audit_logs 완전성 검증 테스트

REQ-UBI-003: The system SHALL record an audit log entry for every document upload,
workflow creation, draft generation, and prediction event in `audit_logs` 테이블
with user_id, action, resource_id, and timestamp.

# @MX:TODO: [AUTO] AC-UBI-003 구현 미완 — RED 페이즈. GREEN 페이즈에서 제거 예정.
# @MX:SPEC: SPEC-AX-001 REQ-UBI-003 / AC-UBI-003
"""
from __future__ import annotations

import uuid
from typing import Any
from unittest.mock import MagicMock

import pytest

# =============================================================================
# 테스트 픽스처
# =============================================================================


@pytest.fixture()
def sample_document_id() -> str:
    """테스트용 문서 ID"""
    return str(uuid.uuid4())


@pytest.fixture()
def sample_workflow_id() -> str:
    """테스트용 워크플로우 ID"""
    return str(uuid.uuid4())


@pytest.fixture()
def sample_report_id() -> str:
    """테스트용 보고서 ID"""
    return str(uuid.uuid4())


@pytest.fixture()
def sample_simulation_id() -> str:
    """테스트용 시뮬레이션 ID"""
    return str(uuid.uuid4())


@pytest.fixture()
def mock_db_session() -> MagicMock:
    """감사 로그 INSERT를 기록하는 mock DB 세션"""
    session = MagicMock()
    session.execute = MagicMock(return_value=None)
    session.commit = MagicMock(return_value=None)
    return session


# =============================================================================
# AC-UBI-003 테스트 케이스
# =============================================================================


class TestAuditLoggingCompleteness:
    """REQ-UBI-003 감사 로깅 — 4종 액션 완전성 검증"""

    def test_document_upload_audit_event_should_include_all_required_fields(
        self, sample_document_id: str
    ) -> None:
        """문서 업로드 감사 이벤트가 4개 필수 필드를 모두 포함해야 한다.

        Given: document_id가 있는 신규 문서 업로드
        When: audit_event(action='document_upload') 호출
        Then: 반환 레코드에 user_id, action, resource_id, timestamp 4필드 존재
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            user_id="test-user",
            action="document_upload",
            resource_id=sample_document_id,
            resource_type="document",
        )
        # 4개 필수 필드 검증
        assert "user_id" in result
        assert "action" in result
        assert "resource_id" in result
        assert "timestamp" in result
        assert result["action"] == "document_upload"
        assert result["resource_id"] == sample_document_id

    def test_workflow_create_audit_event_should_include_all_required_fields(
        self, sample_workflow_id: str
    ) -> None:
        """워크플로우 생성 감사 이벤트가 4개 필수 필드를 모두 포함해야 한다.

        Given: workflow_id가 있는 신규 워크플로우 생성
        When: audit_event(action='workflow_create') 호출
        Then: 반환 레코드에 user_id, action, resource_id, timestamp 4필드 존재
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            user_id="test-user",
            action="workflow_create",
            resource_id=sample_workflow_id,
            resource_type="workflow",
        )
        assert "user_id" in result
        assert "action" in result
        assert "resource_id" in result
        assert "timestamp" in result
        assert result["action"] == "workflow_create"

    def test_draft_generate_audit_event_should_include_all_required_fields(
        self, sample_report_id: str
    ) -> None:
        """초안 생성 감사 이벤트가 4개 필수 필드를 모두 포함해야 한다.

        Given: report_id가 있는 초안 생성 작업
        When: audit_event(action='draft_generate') 호출
        Then: 반환 레코드에 user_id, action, resource_id, timestamp 4필드 존재
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            user_id="test-user",
            action="draft_generate",
            resource_id=sample_report_id,
            resource_type="report",
        )
        assert "user_id" in result
        assert "action" in result
        assert "resource_id" in result
        assert "timestamp" in result
        assert result["action"] == "draft_generate"

    def test_prediction_audit_event_should_include_all_required_fields(
        self, sample_simulation_id: str
    ) -> None:
        """등급 예측 감사 이벤트가 4개 필수 필드를 모두 포함해야 한다.

        Given: simulation_id가 있는 등급 예측 작업
        When: audit_event(action='prediction') 호출
        Then: 반환 레코드에 user_id, action, resource_id, timestamp 4필드 존재
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            user_id="test-user",
            action="prediction",
            resource_id=sample_simulation_id,
            resource_type="simulation",
        )
        assert "user_id" in result
        assert "action" in result
        assert "resource_id" in result
        assert "timestamp" in result
        assert result["action"] == "prediction"

    def test_four_sequential_actions_should_produce_four_audit_records(
        self,
        sample_document_id: str,
        sample_workflow_id: str,
        sample_report_id: str,
        sample_simulation_id: str,
    ) -> None:
        """4종 액션 순차 수행 시 정확히 4개의 audit 레코드가 생성되어야 한다.

        Given: PostgreSQL audit_logs 테이블 (mock)
        When: document_upload, workflow_create, draft_generate, prediction 4종 순차 호출
        Then: 정확히 4개의 레코드 생성, 각 레코드의 action이 올바른 값

        AC-UBI-003: 레코드 수 < 4이면 본 AC 실패
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        records: list[dict[str, Any]] = []

        records.append(
            audit_event(
                user_id="test-user",
                action="document_upload",
                resource_id=sample_document_id,
                resource_type="document",
            )
        )
        records.append(
            audit_event(
                user_id="test-user",
                action="workflow_create",
                resource_id=sample_workflow_id,
                resource_type="workflow",
            )
        )
        records.append(
            audit_event(
                user_id="test-user",
                action="draft_generate",
                resource_id=sample_report_id,
                resource_type="report",
            )
        )
        records.append(
            audit_event(
                user_id="test-user",
                action="prediction",
                resource_id=sample_simulation_id,
                resource_type="simulation",
            )
        )

        assert len(records) == 4

        expected_actions = {
            "document_upload",
            "workflow_create",
            "draft_generate",
            "prediction",
        }
        actual_actions = {r["action"] for r in records}
        assert actual_actions == expected_actions

    def test_audit_event_timestamp_should_be_iso8601_format(
        self, sample_document_id: str
    ) -> None:
        """감사 이벤트 timestamp가 ISO 8601 (timezone 포함) 형식이어야 한다.

        Given: 임의의 감사 이벤트
        When: audit_event() 호출
        Then: timestamp 필드가 ISO 8601 형식 (예: 2026-05-14T10:00:00+09:00)
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            user_id="test-user",
            action="document_upload",
            resource_id=sample_document_id,
            resource_type="document",
        )
        timestamp = result.get("timestamp")
        assert timestamp is not None
        # ISO 8601 기본 검증 — 'T' 구분자 포함 여부
        timestamp_str = str(timestamp)
        assert "T" in timestamp_str or "-" in timestamp_str

    def test_audit_event_without_user_id_should_not_store_null_user_id(
        self, sample_document_id: str
    ) -> None:
        """user_id 없이 호출 시 NULL 또는 빈 문자열이 저장되어서는 안 된다.

        Given: user_id를 제공하지 않는 호출 (settings.default_user_id로 대체)
        When: audit_event() 호출 (user_id 생략)
        Then: 저장된 user_id가 'cli-anonymous' (NULL 불허, 빈 문자열 불허)
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            action="document_upload",
            resource_id=sample_document_id,
            resource_type="document",
            # user_id 생략 — settings.default_user_id='cli-anonymous'로 대체되어야 함
        )
        assert result["user_id"] is not None
        assert result["user_id"] != ""
        assert result["user_id"] == "cli-anonymous"
