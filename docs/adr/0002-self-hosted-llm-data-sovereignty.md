# ADR 0002: 자체 호스팅 LLM (Qwen 2.5) + 망분리 정합

## Status

Accepted (2026-05-14)

## Context

KEPCO E&C 배포 환경 특성:
- **망분리 (Network Segmentation)**: 외부 인터넷 접근 0건 (내부망 100% 격리)
- **PIPA (개인정보보호법)**: 사원 이름, 성과 데이터 국내 반입 금지
- **한국 공공기관 데이터 주권**: 외부 클라우드 API 사용 불가 (OpenAI, Google, Anthropic)
- **PISA (기관 정보보호 수준 진단)**: 접근 제어, 암호화, 감사 로그 필수

LLM 선택지:
- **Path A (채택)**: Self-hosted open-source LLM (Qwen 2.5 7B)
- **Path B**: OpenAI API (외부 인터넷 필요, 국내 규제 위반)
- **Path C**: 전자정부 표준 AI (복잡, 정부망 통합 필수)

## Decision

**Qwen 2.5 7B + ko-sroberta-multitask (임베딩) + Qwen2-VL 7B (VLM)** 자체 호스팅.

근거:
- 망분리 100% 준수 (외부 API 호출 0건)
- Apache 2.0 라이선스 (상용 용도 가능)
- 한국어 성능 우수 (Qwen 한국어 Instruct 튜닝)
- K8s 단일 GPU로 운영 가능 (CPU fallback 지원)
- vLLM 통합 (배치 처리, 페이지드 어텐션)

## Consequences

### 긍정적 영향

- **KEPCO E&C 망분리 정책 100% 준수**
- **데이터 주권 보장**: 모든 입출력 내부 PostgreSQL 저장
- **운영 자유도**: 모델 재훈련, fine-tuning 가능
- **비용 예측성**: API 종량 과금 없음, 자체 인프라 운영 비용만

### 부정적 영향

- **인프라 비용**: vLLM + GPU 운영 비용 (단일 GPU sandbox ~$200/월)
- **성능 제약**: 7B 모델의 한국어 품질이 closed-source 대형 모델보다 낮을 수 있음
- **운영 부담**: 모델 업데이트, 의존성 관리, 보안 패치

### 성능 목표

| 작업 | 목표 | 달성 경로 |
|------|------|----------|
| OCR (HWP/PDF) | <2s/page | vLLM 배치 처리 + Qwen2-VL 7B |
| RAG 검색 | <100ms | pgvector HNSW 인덱스 |
| 초안 생성 | <5s | Qwen 2.5 7B (4-bit quantize) |
| 예측 | <1s | scikit-learn 간단 분류기 |

## References

- SPEC-AX-001 research.md §2.1-2.2 (VLM/LLM 선택 분석)
- `.moai/project/tech.md` §2.1-2.2, §3.1-3.3 (기술 스택)
- `tech.md` §9.1 (망분리 요구사항)
- Qwen 공식 모델 카드: https://huggingface.co/Qwen/Qwen2.5-7B-Instruct
- vLLM 공식 문서: https://docs.vllm.ai/

---

**작성자**: ircp  
**날짜**: 2026-05-14
