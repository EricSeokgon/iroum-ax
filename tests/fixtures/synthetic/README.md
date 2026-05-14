# 합성 픽스처 (Synthetic Fixtures) — REQ-AX-001

SPEC-AX-001 테스트에 사용하는 합성 픽스처 파일 설명.

## 커밋 정책

| 픽스처 유형 | 커밋 여부 | 이유 |
|------------|----------|------|
| 합성 HWP (generate_fixtures.py로 생성) | YES | CI에서 재현 가능 |
| 합성 PDF (reportlab으로 생성) | YES | CI에서 재현 가능 |
| 실제 KEPCO E&C HWP | NO (gitignore) | 고객 기밀 자료 |
| 실제 평가편람 PDF | NO (gitignore) | 고객 기밀 자료 |

실제 데이터 파일은 `.gitignore`에 의해 커밋되지 않습니다:
```
tests/fixtures/*.real.hwp
tests/fixtures/kepco/
```

## 합성 픽스처 목록 (GREEN phase에서 생성 예정)

### sample_report.hwp

- 설명: 5페이지 합성 안전보건 실적보고서 HWP
- 용도: AC-001-1 (정상 HWP 파싱), AC-001-5 (GPU/CPU 분기)
- 생성: `tests/fixtures/generate_fixtures.py` 실행
- 내용:
  - 1페이지: 개요 (한글 텍스트 + 영문 약어 KOSHA, ISO)
  - 2-3페이지: 안전교육 실적 (표 포함)
  - 4페이지: 안전사고 현황 (표 포함)
  - 5페이지: 향후 계획 (텍스트)
- 메타데이터: author="KEPCO E&C 안전보건팀", created_date="2026-01-15"

### corrupted_ole.hwp

- 설명: OLE 헤더가 의도적으로 손상된 HWP
- 용도: AC-001-2 (OLE 손상 → VLM 폴백)
- 생성: sample_report.hwp의 첫 4바이트를 `\xff\xfe\xfd\xfc`로 교체
- 예상 동작: hwp-converter가 OLECompoundError 발생 → VLM OCR 자동 폴백

### rotated_table.pdf

- 설명: 90° 회전된 표 페이지를 포함하는 PDF
- 용도: AC-001-3 (회전 PDF 표 추출 정확도)
- 생성: reportlab 라이브러리로 생성 (페이지 회전 행렬 적용)
- 내용:
  - 1페이지: 정상 방향 텍스트
  - 2페이지: 90° 회전된 2×3 표

## 픽스처 생성 방법 (GREEN phase)

```bash
# 픽스처 생성 스크립트 실행
python tests/fixtures/generate_fixtures.py

# 생성 결과 확인
ls -la tests/fixtures/synthetic/
```

## RED phase 대체 전략

RED phase (현재 단계)에서는 실제 HWP/PDF 파일 없이 `unittest.mock`으로 대체:

```python
# 파서 내부 _load_hwp 메서드를 패치하여 모의 데이터 반환
with patch.object(parser, "_load_hwp", return_value=mock_hwp_doc):
    result = parser.parse(hwp_path)
```

GREEN phase에서 실제 파일을 생성하고 mock 패치를 제거한다.
