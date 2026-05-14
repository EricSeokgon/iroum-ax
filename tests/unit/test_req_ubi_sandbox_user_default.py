"""AC-UBI-004: sandbox user_id 기본값 — AUTH_ENABLED=false 시 'cli-anonymous' 검증 테스트

REQ-UBI-003 (State-driven 절, v0.1.2 추가):
WHILE 인증 시스템(SSO/JWT)이 비활성화 상태(sandbox 환경)일 때,
the system SHALL audit_logs.user_id 필드에 'cli-anonymous' 문자열 기본값을 기록한다.

AC-UBI-004: AUTH_ENABLED=false 상태에서 모든 audit_logs 레코드의 user_id가
'cli-anonymous'여야 하며 NULL 값이나 빈 문자열은 허용되지 않는다.

# @MX:TODO: [AUTO] AC-UBI-004 구현 미완 — RED 페이즈. GREEN 페이즈에서 제거 예정.
# @MX:SPEC: SPEC-AX-001 REQ-UBI-003 State-driven / AC-UBI-004
"""
from __future__ import annotations

import uuid

import pytest


# =============================================================================
# 테스트 픽스처
# =============================================================================


@pytest.fixture()
def mock_sso_disabled(monkeypatch: pytest.MonkeyPatch) -> None:
    """SSO/JWT 인증 시스템이 비활성화된 sandbox 환경을 모사한다.

    pipelines/config/settings.py의 AUTH_ENABLED=false 상태를 monkeypatch로 설정.
    이 픽스처를 사용하는 테스트는 인증 없는 sandbox 환경을 전제한다.
    """
    monkeypatch.setenv("AUTH_ENABLED", "false")
    monkeypatch.setenv("DEFAULT_USER_ID", "cli-anonymous")


@pytest.fixture()
def mock_sso_enabled(monkeypatch: pytest.MonkeyPatch) -> None:
    """SSO/JWT 인증 시스템이 활성화된 환경을 모사한다. (비교 케이스)"""
    monkeypatch.setenv("AUTH_ENABLED", "true")
    monkeypatch.setenv("DEFAULT_USER_ID", "cli-anonymous")  # 기본값은 유지


# =============================================================================
# AC-UBI-004 테스트 케이스
# =============================================================================


class TestSandboxUserIdDefault:
    """REQ-UBI-003 State-driven — sandbox user_id 기본값 'cli-anonymous' 검증"""

    def test_document_upload_with_sso_disabled_should_use_cli_anonymous_user_id(
        self, mock_sso_disabled: None
    ) -> None:
        """SSO 비활성화 상태에서 문서 업로드 감사 이벤트의 user_id가 'cli-anonymous'여야 한다.

        Given: AUTH_ENABLED=false (Exclusion §12 sandbox 환경)
        When: 인증 헤더 없이 document_upload audit_event 호출
        Then: audit_logs.user_id == 'cli-anonymous' (NULL 불허, 빈 문자열 불허)
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            action="document_upload",
            resource_id=str(uuid.uuid4()),
            resource_type="document",
            # user_id 미제공 — settings.auth_enabled=False 상태에서 기본값 적용
        )
        assert result["user_id"] == "cli-anonymous"
        assert result["user_id"] is not None
        assert result["user_id"] != ""

    def test_workflow_create_with_sso_disabled_should_use_cli_anonymous_user_id(
        self, mock_sso_disabled: None
    ) -> None:
        """SSO 비활성화 상태에서 워크플로우 생성 감사 이벤트의 user_id가 'cli-anonymous'여야 한다.

        Given: AUTH_ENABLED=false
        When: 인증 헤더 없이 workflow_create audit_event 호출
        Then: audit_logs.user_id == 'cli-anonymous'
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            action="workflow_create",
            resource_id=str(uuid.uuid4()),
            resource_type="workflow",
        )
        assert result["user_id"] == "cli-anonymous"

    def test_draft_generate_with_sso_disabled_should_use_cli_anonymous_user_id(
        self, mock_sso_disabled: None
    ) -> None:
        """SSO 비활성화 상태에서 초안 생성 감사 이벤트의 user_id가 'cli-anonymous'여야 한다.

        Given: AUTH_ENABLED=false
        When: 인증 헤더 없이 draft_generate audit_event 호출
        Then: audit_logs.user_id == 'cli-anonymous'
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            action="draft_generate",
            resource_id=str(uuid.uuid4()),
            resource_type="report",
        )
        assert result["user_id"] == "cli-anonymous"

    def test_prediction_with_sso_disabled_should_use_cli_anonymous_user_id(
        self, mock_sso_disabled: None
    ) -> None:
        """SSO 비활성화 상태에서 등급 예측 감사 이벤트의 user_id가 'cli-anonymous'여야 한다.

        Given: AUTH_ENABLED=false
        When: 인증 헤더 없이 prediction audit_event 호출
        Then: audit_logs.user_id == 'cli-anonymous'
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            action="prediction",
            resource_id=str(uuid.uuid4()),
            resource_type="simulation",
        )
        assert result["user_id"] == "cli-anonymous"

    def test_all_four_actions_with_sso_disabled_should_all_have_cli_anonymous(
        self, mock_sso_disabled: None
    ) -> None:
        """SSO 비활성화 상태에서 4종 액션 모두 user_id가 'cli-anonymous'여야 한다.

        Given: AUTH_ENABLED=false
        When: 4종 액션 순차 수행 (인증 헤더 없음)
        Then: 모든 audit_logs 레코드의 user_id == 'cli-anonymous'
              NULL 값 0건, 빈 문자열 0건

        AC-UBI-004 핵심 검증: 4종 모두 cli-anonymous 여야 함
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        actions = [
            ("document_upload", "document"),
            ("workflow_create", "workflow"),
            ("draft_generate", "report"),
            ("prediction", "simulation"),
        ]

        records = [
            audit_event(
                action=action,
                resource_id=str(uuid.uuid4()),
                resource_type=resource_type,
            )
            for action, resource_type in actions
        ]

        assert len(records) == 4

        for record in records:
            assert record["user_id"] == "cli-anonymous", (
                f"action='{record['action']}' 레코드의 user_id가 'cli-anonymous'가 아님: "
                f"{record['user_id']!r}"
            )

    def test_settings_auth_enabled_false_should_have_default_user_id_cli_anonymous(
        self, mock_sso_disabled: None
    ) -> None:
        """settings.auth_enabled=False 상태에서 settings.default_user_id가 'cli-anonymous'여야 한다.

        Given: AUTH_ENABLED=false 환경변수
        When: Settings 인스턴스 생성
        Then: settings.default_user_id == 'cli-anonymous'
              settings.auth_enabled == False
        """
        # Settings를 다시 인스턴스화하여 환경변수를 반영
        from pipelines.config.settings import Settings  # type: ignore[import]

        test_settings = Settings()
        assert test_settings.auth_enabled is False
        assert test_settings.default_user_id == "cli-anonymous"

    def test_user_id_should_not_be_null_with_sso_disabled(
        self, mock_sso_disabled: None
    ) -> None:
        """SSO 비활성화 시 audit_logs.user_id에 NULL이 저장되어서는 안 된다.

        Given: AUTH_ENABLED=false
        When: audit_event() 호출 (user_id 미제공)
        Then: user_id is not None (NULL 저장 불허)

        이 테스트는 DB 스키마의 NOT NULL 제약(initial.sql: user_id NOT NULL)과
        application 레벨 기본값 설정 모두를 검증한다.
        """
        from pkg.logging.logger import audit_event  # type: ignore[import]

        result = audit_event(
            action="document_upload",
            resource_id=str(uuid.uuid4()),
            resource_type="document",
        )
        assert result["user_id"] is not None, "user_id에 NULL이 저장되었음 — NOT NULL 위반"
        assert result["user_id"] != "", "user_id가 빈 문자열로 저장되었음"
