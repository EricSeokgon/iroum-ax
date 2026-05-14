"""합성 데모 픽스처 생성기 — iroum-ax Walking Skeleton PoC 시연용

이 스크립트가 생성하는 모든 데이터는 가상(합성)입니다.
실제 KEPCO E&C 자료나 타 기관 보고서와 무관합니다.

Usage:
    python pipelines/scripts/gen_synthetic_data.py [--output-dir DIR] [--force]
"""
from __future__ import annotations

import argparse
import json
import logging
import sys
from pathlib import Path

# # @MX:NOTE: [AUTO] 합성 데이터 생성 진입점 — demo-fixtures make 타겟에서 호출됨
# # @MX:SPEC: SPEC-AX-001 (시연용 픽스처, 운영 코드 아님)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
logger = logging.getLogger(__name__)

# ============================================================
# 픽스처 데이터 정의
# ============================================================

_CRITERIA_FIXTURE: dict = {
    "category": "안전보건",
    "year": 2026,
    "description": "공공기관 경영평가 안전보건 부문 평가기준 (가상 데이터)",
    "note": "본 데이터는 시연용 합성 데이터이며 실제 기획재정부 평가편람과 상이할 수 있습니다.",
    "total_weight": 100,
    "indicators": [
        {
            "id": "SH-1",
            "name": "안전교육 이수율",
            "weight": 30,
            "description": "신규 입사자 및 전사원 대상 법정·자체 안전교육 이수율 관리",
            "sub_criteria": [
                {
                    "id": "SH-1-1",
                    "name": "신규 입사자 안전교육 이수율",
                    "detail": "신규 입사 후 1개월 이내 100% 이수 달성 여부",
                    "max_points": 15,
                    "scoring_guide": {
                        "A": "이수율 100% 달성, 교육 내용 충실, 평가 시행",
                        "B": "이수율 95-99%, 교육 내용 보통",
                        "C": "이수율 90-94%",
                        "D": "이수율 90% 미만",
                    },
                },
                {
                    "id": "SH-1-2",
                    "name": "정기 안전교육 이수율",
                    "detail": "연 2회 이상 전사원 정기 안전교육 실시 및 이수율",
                    "max_points": 15,
                    "scoring_guide": {
                        "A": "이수율 98% 이상, 교육 주기 준수, 온오프 혼합",
                        "B": "이수율 90-97%",
                        "C": "이수율 85-89%",
                        "D": "이수율 85% 미만",
                    },
                },
            ],
        },
        {
            "id": "SH-2",
            "name": "중대재해 예방투자",
            "weight": 25,
            "description": "중대산업재해 예방을 위한 안전투자 현황 및 중대재해 발생 현황",
            "sub_criteria": [
                {
                    "id": "SH-2-1",
                    "name": "안전보건 투자 규모",
                    "detail": "연간 안전보건 예산 투자액 및 전년 대비 증가율",
                    "max_points": 10,
                    "scoring_guide": {
                        "A": "투자액 50억 이상 또는 전년 대비 20% 이상 증가",
                        "B": "투자액 20-50억 또는 10-19% 증가",
                        "C": "투자액 10-20억 또는 0-9% 증가",
                        "D": "투자액 10억 미만 또는 전년 대비 감소",
                    },
                },
                {
                    "id": "SH-2-2",
                    "name": "중대재해 발생 현황",
                    "detail": "최근 3년간 중대산업재해(사망·중상) 발생 건수",
                    "max_points": 15,
                    "scoring_guide": {
                        "A": "3년 연속 중대재해 Zero",
                        "B": "1건 이하 발생, 재발방지 대책 우수",
                        "C": "2건 이하 발생",
                        "D": "3건 이상 발생",
                    },
                },
            ],
        },
        {
            "id": "SH-3",
            "name": "정기안전 점검",
            "weight": 20,
            "description": "법정 정기안전점검 이행률 및 자체 점검 실시 현황",
            "sub_criteria": [
                {
                    "id": "SH-3-1",
                    "name": "법정 정기점검 이행률",
                    "detail": "산업안전보건법 기준 법정 안전점검 100% 이행 여부",
                    "max_points": 10,
                    "scoring_guide": {
                        "A": "법정점검 100% 이행, 지적사항 즉시 개선",
                        "B": "95-99% 이행",
                        "C": "90-94% 이행",
                        "D": "90% 미만 이행",
                    },
                },
                {
                    "id": "SH-3-2",
                    "name": "자체 안전점검 시행",
                    "detail": "자체 월별/분기별 안전점검 실시 현황 및 개선 이행률",
                    "max_points": 10,
                    "scoring_guide": {
                        "A": "월별 점검 + 분기 심화점검, 개선율 95% 이상",
                        "B": "분기별 점검, 개선율 80% 이상",
                        "C": "반기별 점검",
                        "D": "연 1회 이하",
                    },
                },
            ],
        },
        {
            "id": "SH-4",
            "name": "안전보건 경영체계",
            "weight": 15,
            "description": "ISO 45001 또는 동등 수준 안전보건경영시스템 구축·운영",
            "sub_criteria": [
                {
                    "id": "SH-4-1",
                    "name": "안전보건경영시스템 인증",
                    "detail": "ISO 45001 인증 또는 KOSHA-MS 인증 취득 및 유지",
                    "max_points": 10,
                    "scoring_guide": {
                        "A": "ISO 45001 인증 취득 및 지속 유지, 개선활동 우수",
                        "B": "KOSHA-MS 인증 취득",
                        "C": "인증 준비 중 (심사 예정)",
                        "D": "인증 미취득",
                    },
                },
                {
                    "id": "SH-4-2",
                    "name": "위험성 평가 이행",
                    "detail": "산업안전보건법 위험성 평가 연 1회 이상 실시",
                    "max_points": 5,
                    "scoring_guide": {
                        "A": "연 2회 이상 실시, 위험요인 개선 90% 이상",
                        "B": "연 1회 실시, 개선 70% 이상",
                        "C": "실시하였으나 개선 미흡",
                        "D": "미실시",
                    },
                },
            ],
        },
        {
            "id": "SH-5",
            "name": "안전보건 문화",
            "weight": 10,
            "description": "임직원 안전의식 제고 및 안전보건 문화 활동",
            "sub_criteria": [
                {
                    "id": "SH-5-1",
                    "name": "안전보건 캠페인·행사",
                    "detail": "안전보건의 달 등 전사 캠페인 실시 여부",
                    "max_points": 5,
                    "scoring_guide": {
                        "A": "연 4회 이상 전사 캠페인, 참여율 80% 이상",
                        "B": "연 2-3회 캠페인",
                        "C": "연 1회 캠페인",
                        "D": "미실시",
                    },
                },
                {
                    "id": "SH-5-2",
                    "name": "안전 제안 제도",
                    "detail": "임직원 안전 개선 제안 활성화 현황",
                    "max_points": 5,
                    "scoring_guide": {
                        "A": "연 50건 이상 제안 접수·반영, 포상 제도 운영",
                        "B": "연 20-49건 제안 접수",
                        "C": "연 10-19건",
                        "D": "연 10건 미만",
                    },
                },
            ],
        },
    ],
}

_GUIDELINES_FIXTURE: str = """공공기관 경영평가 보고서 작성지침 — 합성 가이드

[주의] 이 문서는 시연용 합성 데이터입니다. 실제 기획재정부 작성지침과 상이할 수 있습니다.

================================================================
1. 기본 원칙
================================================================

1.1 문체 및 경어체
- 모든 본문은 '합니다체' (합니다, 하였습니다, 입니다 등) 사용 필수
- '해요체', '해체', '합시오체' 사용 금지
- 예시 (올바름): "당 기관은 안전교육을 실시하였습니다."
- 예시 (잘못됨): "당 기관은 안전교육을 실시했어요." / "실시했다."

1.2 주어
- 기관을 지칭할 때: "당 기관", "본 기관", "귀 기관"(타 기관 지칭시) 사용
- "우리 기관", "저희" 사용 가능하나 가급적 공식 명칭 사용 권장

1.3 표제어 및 제목
- 장(章): 1. 2. 3. (아라비아 숫자 + 마침표)
- 절(節): 1.1 1.2 (점표기)
- 항(項): 가. 나. 다. (한글 가나다 + 마침표)
- 목(目): (1) (2) (3) (괄호 아라비아 숫자)

================================================================
2. 서술 방식
================================================================

2.1 사실 중심 서술
- 정량 지표 반드시 포함: 수치, 비율, 건수 등
- 예시: "안전교육 이수율 98% 달성 (목표 100% 대비 98% 수준)"
- 막연한 표현 지양: "다수의", "많은" → "OO건", "OO%"

2.2 시제
- 기 완료된 사항: 과거형 사용 ("실시하였습니다", "달성하였습니다")
- 진행 중이거나 계획된 사항: 현재진행형 또는 미래형 ("추진하고 있습니다", "추진할 계획입니다")

2.3 증빙 및 근거
- 가능한 경우 증빙 자료 번호 또는 첨부 자료 참조 표기
- 예시: "자세한 사항은 [별첨 1] 안전교육 이수 현황을 참조하시기 바랍니다."

================================================================
3. 금지 사항
================================================================

3.1 금지 표현
- 외부 LLM/AI 생성 흔적이 명백한 문구 금지 (예: "물론입니다", "확실히", "물론이죠")
- 과도한 수식어 남발 금지 ("매우 훌륭한", "탁월한")
- 불확실한 사실을 단정 짓는 표현 금지

3.2 형식적 오류
- 동일 문장 반복 금지
- 개조식(~함, ~됨)과 완성형(합니다체) 혼용 금지
- 표 내부와 본문의 수치 불일치 금지

================================================================
4. 평가지표 서술 순서 (권장)
================================================================

① 추진 배경 및 목적 (2-3문장)
② 주요 추진 내용 및 실적 (수치 포함, 3-5문장)
③ 성과 및 효과 (정량 성과 우선, 2-3문장)
④ 한계 및 개선계획 (선택사항, 1-2문장)

================================================================
5. 분량 기준
================================================================

- 각 지표별 본문: A4 1/2~1페이지 (500~1,000자)
- 표, 그림 포함 시: 본문 약 300자 + 표/그림 가능
- 첨부자료: 별도 첨부 (본문 분량 미산정)
"""

_KEPCO_SELF_REPORT_FIXTURE: dict = {
    "report_id": "KEPCO-2026-SH",
    "organization": "한국전력기술(주) (가상 데이터)",
    "report_year": 2026,
    "evaluation_category": "안전보건",
    "note": "본 보고서는 시연용 합성 가상 데이터입니다. 실제 KEPCO 자료와 무관합니다.",
    "predicted_grade": "B",
    "content": {
        "summary": (
            "당 기관은 2025년 안전보건 강화를 위하여 다양한 활동을 추진하였습니다. "
            "안전교육 이수율 향상, 사고예방 투자 확대, 정기점검 내실화 등을 통하여 "
            "안전보건 수준 제고에 노력하였습니다."
        ),
        "indicators": [
            {
                "criterion_id": "SH-1",
                "name": "안전교육 이수율",
                "actual_value": "78%",
                "target_value": "95%",
                "gap": "17%p 미달",
                "content": (
                    "당 기관은 2025년 안전교육 이수율 향상을 위하여 온라인 교육 플랫폼을 "
                    "도입하였습니다. 신규 입사자 대상 안전교육은 78%의 이수율을 기록하였으며, "
                    "이는 목표치 100% 대비 미흡한 수준입니다. 일부 현장직 직원의 교육 이수가 "
                    "지연되었으며, 이에 대한 원인 분석 및 개선방안을 수립하고 있습니다. "
                    "2026년에는 교육 이수율 90% 이상 달성을 목표로 교육 방식을 다양화하고 "
                    "현장 접근성을 높이는 방향으로 추진할 계획입니다."
                ),
                "evidence": "별첨 1. 안전교육 이수 현황표",
            },
            {
                "criterion_id": "SH-2",
                "name": "중대재해 예방투자",
                "actual_value": "15억 원",
                "target_value": "30억 원 이상",
                "gap": "투자 규모 미흡",
                "content": (
                    "당 기관은 2025년 안전보건 예산으로 약 15억 원을 투자하였습니다. "
                    "주요 투자 항목으로는 안전장비 교체(8억 원), 안전시설 개선(4억 원), "
                    "안전교육 프로그램 개발(3억 원) 등이 있습니다. 중대산업재해는 2025년 "
                    "1건 발생하였으며(경상), 즉각 원인조사 및 재발방지 대책을 수립하였습니다. "
                    "2026년에는 투자액을 20억 원으로 확대하고 위험 공정 집중 개선을 추진할 "
                    "계획입니다."
                ),
                "evidence": "별첨 2. 안전보건 투자 내역",
            },
            {
                "criterion_id": "SH-3",
                "name": "정기안전 점검",
                "actual_value": "분기 1회",
                "target_value": "월 1회 + 분기 심화",
                "gap": "점검 주기 부족",
                "content": (
                    "당 기관은 산업안전보건법에 따른 법정 정기안전점검을 분기별 1회 실시하여 "
                    "법정 기준을 준수하였습니다. 점검 결과 총 23건의 지적사항이 발생하였으며 "
                    "이 중 18건(78%)을 개선 완료하였습니다. A 등급 기관 대비 자체 점검 "
                    "빈도가 낮은 것이 과제로 식별되었으며, 2026년에는 월별 자체 점검을 "
                    "도입할 계획입니다."
                ),
                "evidence": "별첨 3. 정기안전점검 결과 보고서",
            },
            {
                "criterion_id": "SH-4",
                "name": "안전보건 경영체계",
                "actual_value": "KOSHA-MS 인증 준비 중",
                "target_value": "ISO 45001 인증 취득",
                "gap": "인증 미취득",
                "content": (
                    "당 기관은 현재 KOSHA-MS 인증 취득을 위한 준비 절차를 진행 중에 있습니다. "
                    "2025년 내부 심사를 완료하였으며, 2026년 상반기 중 외부 인증 심사를 "
                    "신청할 예정입니다. 위험성 평가는 2025년 연 1회 실시하여 총 45개 위험요인을 "
                    "파악하였으며, 이 중 32개(71%)를 개선 완료하였습니다."
                ),
                "evidence": "별첨 4. 위험성평가 결과표",
            },
            {
                "criterion_id": "SH-5",
                "name": "안전보건 문화",
                "actual_value": "연 2회 캠페인",
                "target_value": "연 4회 이상",
                "gap": "캠페인 횟수 부족",
                "content": (
                    "당 기관은 2025년 안전보건의 날 캠페인(5월)과 직장 내 안전문화 강화 "
                    "캠페인(11월) 등 총 2회의 전사 안전캠페인을 실시하였습니다. 임직원 참여율은 "
                    "평균 65% 수준이었습니다. 안전 제안 제도를 통해 12건의 개선 제안이 접수되었으며, "
                    "이 중 8건을 반영하였습니다. 향후 캠페인 횟수를 분기별 1회로 확대하고 "
                    "참여 인센티브를 강화할 계획입니다."
                ),
                "evidence": "별첨 5. 안전캠페인 실시 현황",
            },
        ],
    },
    "grade_scores": {
        "SH-1": {"score": 18, "max": 30, "grade": "B"},
        "SH-2": {"score": 15, "max": 25, "grade": "C"},
        "SH-3": {"score": 13, "max": 20, "grade": "B"},
        "SH-4": {"score": 8, "max": 15, "grade": "C"},
        "SH-5": {"score": 6, "max": 10, "grade": "B"},
        "total": {"score": 60, "max": 100, "grade": "B"},
    },
}

_A_GRADE_REPORT_FIXTURE: dict = {
    "report_id": "OTHER-A-2026",
    "organization": "가상 공기업 A사 (합성 가상 데이터)",
    "report_year": 2026,
    "evaluation_category": "안전보건",
    "note": "본 보고서는 시연용 합성 가상 데이터입니다. 실제 기관 자료와 무관합니다.",
    "predicted_grade": "A",
    "content": {
        "summary": (
            "당 기관은 2025년 안전보건 분야에서 탁월한 성과를 거두었습니다. "
            "안전교육 이수율 100% 달성, 중대재해 Zero 3년 연속 유지, ISO 45001 인증 "
            "유지 등 전 지표에서 우수한 수준을 달성하였습니다."
        ),
        "indicators": [
            {
                "criterion_id": "SH-1",
                "name": "안전교육 이수율",
                "actual_value": "100%",
                "target_value": "100%",
                "gap": "목표 달성",
                "content": (
                    "당 기관은 2025년 신규 입사자 및 전 직원 대상 안전교육 이수율 100%를 달성하였습니다. "
                    "분기별 집합교육 및 온라인 자율학습 혼합 방식을 도입하여 현장 직원의 접근성을 "
                    "획기적으로 개선하였습니다. 안전교육 내용은 최신 법령 개정사항을 반영하여 "
                    "연 2회 이상 업데이트하였으며, 교육 이수 후 평가를 통해 실질적인 학습 효과를 "
                    "검증하였습니다. 특히 신규 입사자는 입사 당일부터 2주 집중 안전교육 프로그램을 "
                    "이수하도록 의무화하였습니다."
                ),
                "evidence": "별첨 1. 안전교육 이수 현황 및 평가결과",
            },
            {
                "criterion_id": "SH-2",
                "name": "중대재해 예방투자",
                "actual_value": "52억 원",
                "target_value": "50억 원 이상",
                "gap": "목표 초과 달성",
                "content": (
                    "당 기관은 2025년 안전보건 분야에 52억 원을 투자하여 전년(45억 원) 대비 "
                    "15.6% 증가하였습니다. 위험 공정 자동화 설비 도입(25억 원), 노후 안전장비 "
                    "전면 교체(15억 원), 스마트 안전관리 시스템 구축(12억 원) 등에 집중 투자하였습니다. "
                    "중대산업재해는 2023년부터 2025년까지 3년 연속 Zero를 달성하였으며, "
                    "이는 지속적인 예방 투자의 성과로 평가됩니다."
                ),
                "evidence": "별첨 2. 안전보건 투자 내역 및 중대재해 현황",
            },
            {
                "criterion_id": "SH-3",
                "name": "정기안전 점검",
                "actual_value": "월 1회 + 분기 심화",
                "target_value": "월 1회 + 분기 심화",
                "gap": "목표 달성",
                "content": (
                    "당 기관은 법정 정기안전점검을 100% 이행하는 것을 기반으로, 자체적으로 "
                    "월별 정기점검과 분기별 심화점검을 추가 실시하였습니다. 2025년 총 168회의 "
                    "점검을 실시하였으며, 지적사항 97건 중 95건(97.9%)을 지적일 기준 30일 이내에 "
                    "개선 완료하였습니다. 점검 결과는 디지털 플랫폼을 통해 실시간으로 관리하며, "
                    "CEO 대시보드에서 월별 현황을 직접 확인할 수 있는 체계를 구축하였습니다."
                ),
                "evidence": "별첨 3. 정기안전점검 결과 및 개선 이행 현황",
            },
            {
                "criterion_id": "SH-4",
                "name": "안전보건 경영체계",
                "actual_value": "ISO 45001 인증 유지 (3년차)",
                "target_value": "ISO 45001 인증 유지",
                "gap": "목표 달성",
                "content": (
                    "당 기관은 ISO 45001:2018 인증을 2023년 최초 취득 후 2025년까지 3년 "
                    "연속 유지하였습니다. 연간 내부 심사 2회, 외부 감시 심사 1회를 실시하여 "
                    "지속적인 관리 수준을 검증하고 있습니다. 위험성 평가는 연 2회(상하반기) "
                    "실시하여 총 78개 위험요인을 파악하였으며, 이 중 74개(94.9%)를 개선 완료하였습니다. "
                    "잔여 4개는 중장기 시설 투자와 연계하여 2026년 상반기까지 개선 예정입니다."
                ),
                "evidence": "별첨 4. ISO 45001 인증서 및 위험성평가 결과",
            },
            {
                "criterion_id": "SH-5",
                "name": "안전보건 문화",
                "actual_value": "연 5회 캠페인, 참여율 88%",
                "target_value": "연 4회 이상, 참여율 80% 이상",
                "gap": "목표 초과 달성",
                "content": (
                    "당 기관은 2025년 분기별 캠페인 외에 특별 행사(중대재해처벌법 1주년 기념 "
                    "안전의식 고취 행사)를 추가하여 총 5회의 전사 안전캠페인을 실시하였습니다. "
                    "임직원 평균 참여율은 88%로 목표(80%)를 초과 달성하였습니다. 안전 제안 제도를 "
                    "통해 연간 73건의 제안이 접수되었으며 이 중 68건(93%)을 반영하고 우수 제안자에 "
                    "대한 포상을 실시하였습니다. 이러한 노력을 인정받아 고용노동부 '안전문화 우수기관'에 "
                    "선정되었습니다."
                ),
                "evidence": "별첨 5. 안전캠페인 결과 및 제안 현황",
            },
        ],
    },
    "grade_scores": {
        "SH-1": {"score": 29, "max": 30, "grade": "A"},
        "SH-2": {"score": 24, "max": 25, "grade": "A"},
        "SH-3": {"score": 19, "max": 20, "grade": "A"},
        "SH-4": {"score": 14, "max": 15, "grade": "A"},
        "SH-5": {"score": 10, "max": 10, "grade": "A"},
        "total": {"score": 96, "max": 100, "grade": "A"},
    },
}

_B_GRADE_REPORT_FIXTURE: dict = {
    "report_id": "OTHER-B-2026",
    "organization": "가상 공기업 B사 (합성 가상 데이터)",
    "report_year": 2026,
    "evaluation_category": "안전보건",
    "note": "본 보고서는 시연용 합성 가상 데이터입니다. 실제 기관 자료와 무관합니다.",
    "predicted_grade": "B",
    "content": {
        "summary": (
            "당 기관은 2025년 안전보건 주요 지표에서 전년 대비 소폭 개선된 실적을 "
            "기록하였습니다. 안전교육 이수율 향상, 정기점검 이행 등 기본 사항은 충족하였으나, "
            "예방 투자 규모 확대 및 경영체계 고도화 부문에서 추가 노력이 필요합니다."
        ),
        "indicators": [
            {
                "criterion_id": "SH-1",
                "name": "안전교육 이수율",
                "actual_value": "91%",
                "target_value": "95%",
                "gap": "4%p 미달",
                "content": (
                    "당 기관은 2025년 안전교육 이수율 91%를 기록하였습니다. 전년(88%) 대비 "
                    "3%p 향상되었으나 목표치 95%에는 미치지 못하였습니다. 온라인 교육 시스템 "
                    "접근성 개선을 통해 이수율을 제고하였으며, 미이수자에 대한 개별 독려 활동을 "
                    "강화하였습니다. 2026년에는 이수율 95% 이상 달성을 목표로 현장 맞춤형 "
                    "교육을 확대할 계획입니다."
                ),
                "evidence": "별첨 1. 안전교육 이수 현황",
            },
            {
                "criterion_id": "SH-2",
                "name": "중대재해 예방투자",
                "actual_value": "21억 원",
                "target_value": "30억 원 이상",
                "gap": "투자 규모 보통",
                "content": (
                    "당 기관은 2025년 안전보건 예산으로 21억 원을 투자하였습니다. "
                    "주요 투자 항목은 안전장비 교체(12억 원)와 교육 프로그램(9억 원)입니다. "
                    "중대산업재해는 1건 발생하였으며 즉각 원인 분석 및 재발방지 대책을 수립하였습니다. "
                    "향후 2년간 단계적으로 투자를 확대하여 30억 원 수준에 도달하는 것을 "
                    "목표로 하고 있습니다."
                ),
                "evidence": "별첨 2. 안전보건 투자 현황",
            },
            {
                "criterion_id": "SH-3",
                "name": "정기안전 점검",
                "actual_value": "분기 1회 + 반기 심화",
                "target_value": "월 1회 + 분기 심화",
                "gap": "점검 주기 미흡",
                "content": (
                    "당 기관은 법정 정기안전점검을 100% 이행하였으며, 추가로 반기별 심화 점검을 "
                    "실시하였습니다. 2025년 점검 결과 지적사항 51건 중 42건(82%)을 개선 완료하였습니다. "
                    "월별 점검 도입 필요성을 인식하고 2026년 시범 운영을 추진할 예정입니다."
                ),
                "evidence": "별첨 3. 정기안전점검 실시 결과",
            },
            {
                "criterion_id": "SH-4",
                "name": "안전보건 경영체계",
                "actual_value": "KOSHA-MS 인증 취득",
                "target_value": "ISO 45001 인증",
                "gap": "ISO 45001 미취득",
                "content": (
                    "당 기관은 2025년 KOSHA-MS 인증을 취득하였습니다. ISO 45001 인증으로의 "
                    "전환을 위한 내부 검토를 완료하였으며, 2026년 ISO 45001 인증 심사를 "
                    "신청할 계획입니다. 위험성 평가는 연 1회 실시하여 38개 위험요인 중 "
                    "29개(76%)를 개선 완료하였습니다."
                ),
                "evidence": "별첨 4. KOSHA-MS 인증서 및 위험성평가 결과",
            },
            {
                "criterion_id": "SH-5",
                "name": "안전보건 문화",
                "actual_value": "연 3회 캠페인, 참여율 72%",
                "target_value": "연 4회 이상",
                "gap": "캠페인 횟수 1회 부족",
                "content": (
                    "당 기관은 2025년 안전보건의 날(5월), 여름철 폭염 대비 캠페인(7월), "
                    "겨울철 안전 캠페인(12월) 등 총 3회의 캠페인을 실시하였습니다. "
                    "임직원 참여율은 72%로 목표 대비 미흡하였습니다. 안전 제안 제도를 통해 "
                    "26건의 제안이 접수되었으며 이 중 20건(77%)을 반영하였습니다."
                ),
                "evidence": "별첨 5. 안전캠페인 실시 결과",
            },
        ],
    },
    "grade_scores": {
        "SH-1": {"score": 22, "max": 30, "grade": "B"},
        "SH-2": {"score": 16, "max": 25, "grade": "B"},
        "SH-3": {"score": 14, "max": 20, "grade": "B"},
        "SH-4": {"score": 9, "max": 15, "grade": "B"},
        "SH-5": {"score": 7, "max": 10, "grade": "B"},
        "total": {"score": 68, "max": 100, "grade": "B"},
    },
}


# ============================================================
# 생성 함수
# ============================================================


def _write_json(path: Path, data: dict, *, force: bool) -> bool:
    """JSON 픽스처 파일을 기록한다. 이미 존재하고 force=False이면 스킵한다."""
    if path.exists() and not force:
        logger.info("  스킵 (이미 존재): %s", path)
        return False
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")
    logger.info("  생성 완료: %s (%d bytes)", path, path.stat().st_size)
    return True


def _write_text(path: Path, content: str, *, force: bool) -> bool:
    """텍스트 픽스처 파일을 기록한다."""
    if path.exists() and not force:
        logger.info("  스킵 (이미 존재): %s", path)
        return False
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    logger.info("  생성 완료: %s (%d bytes)", path, path.stat().st_size)
    return True


def generate_all(output_dir: Path, *, force: bool = False) -> dict[str, int]:
    """모든 합성 픽스처를 생성한다.

    # @MX:ANCHOR: [AUTO] generate_all — Makefile demo-fixtures 및 run_demo.py에서 호출
    # @MX:REASON: 데모 픽스처 생성 진입점, fan_in >= 2 (Makefile + run_demo.py)

    Returns:
        카테고리별 생성/스킵 카운트 딕셔너리
    """
    counts: dict[str, int] = {"created": 0, "skipped": 0}

    logger.info("=== 합성 픽스처 생성 시작 (output_dir=%s) ===", output_dir)
    logger.info("[주의] 모든 데이터는 시연용 가상 데이터입니다.")

    # 1. 평가편람
    logger.info("[1/5] 안전보건 평가편람 생성...")
    created = _write_json(
        output_dir / "criteria" / "안전보건_평가편람.json",
        _CRITERIA_FIXTURE,
        force=force,
    )
    counts["created" if created else "skipped"] += 1

    # 2. 작성지침
    logger.info("[2/5] 작성지침 생성...")
    created = _write_text(
        output_dir / "guidelines" / "작성지침.txt",
        _GUIDELINES_FIXTURE,
        force=force,
    )
    counts["created" if created else "skipped"] += 1

    # 3. KEPCO 자사 보고서
    logger.info("[3/5] KEPCO 자사 안전보건 실적보고서 생성 (B 등급 가정)...")
    created = _write_json(
        output_dir / "reports" / "kepco-self-report-2026.json",
        _KEPCO_SELF_REPORT_FIXTURE,
        force=force,
    )
    counts["created" if created else "skipped"] += 1

    # 4. A 등급 벤치마크
    logger.info("[4/5] 타기관 A 등급 벤치마크 보고서 생성...")
    created = _write_json(
        output_dir / "reports" / "other-A-grade-2026.json",
        _A_GRADE_REPORT_FIXTURE,
        force=force,
    )
    counts["created" if created else "skipped"] += 1

    # 5. B 등급 벤치마크
    logger.info("[5/5] 타기관 B 등급 벤치마크 보고서 생성...")
    created = _write_json(
        output_dir / "reports" / "other-B-grade-2026.json",
        _B_GRADE_REPORT_FIXTURE,
        force=force,
    )
    counts["created" if created else "skipped"] += 1

    logger.info(
        "=== 픽스처 생성 완료 — 생성 %d건, 스킵 %d건 ===",
        counts["created"],
        counts["skipped"],
    )
    return counts


def main() -> int:
    parser = argparse.ArgumentParser(
        description="iroum-ax 데모용 합성 픽스처 생성기 (모든 데이터는 가상)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="예시:\n  python pipelines/scripts/gen_synthetic_data.py\n"
        "  python pipelines/scripts/gen_synthetic_data.py --force",
    )
    parser.add_argument(
        "--output-dir",
        default="tests/fixtures/synthetic",
        help="픽스처 출력 디렉토리 (기본값: tests/fixtures/synthetic)",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="기존 파일 덮어쓰기",
    )
    args = parser.parse_args()

    output_dir = Path(args.output_dir)
    counts = generate_all(output_dir, force=args.force)

    if counts["created"] == 0 and counts["skipped"] > 0:
        logger.info("모든 픽스처가 이미 존재합니다. --force 옵션으로 재생성 가능합니다.")

    return 0


if __name__ == "__main__":
    sys.exit(main())
