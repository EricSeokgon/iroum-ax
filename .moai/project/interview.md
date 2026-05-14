# Project Interview — iroum-ax

> Discovery 인터뷰 결과. `manager-docs`가 product.md / structure.md / tech.md를 생성할 때 핵심 입력으로 사용합니다.
> 기록 일자: 2026-05-14
> 인터뷰 방법: MoAI Socratic Interview (2 라운드 + 비전 폭 결정)

---

## 0. Anchor Use Case (사용자 제공 실제 사례)

**1차 anchor 고객**: 한국전력기술 (KEPCO E&C)

**Use Case**: 공기업 경영평가 보고서 작성·추천 자동화 PoC

**고객 통점(Pain Point)**:
- 경영평가팀이 1년 내내 한 업무에 매달림
- LLM(VLM) 활용한 업무 생산성 향상이 목표

**평가 기준 출처**:
- 정부 제시 "경영평가 편람"
- "작성지침"
- "정권의 정책 가이드" (AI 적용 / 안전 등 정책 우선순위)

**고객이 Agent로 원하는 기능**:
1. 레거시 문서/데이터를 활용한 '경영평가 실적보고서' **초안 작성** (완벽한 문서 아님, 사람 검수 전제)
2. 경영평가 **점수 상승을 위한 Recommendation**

**핵심 기술 도전**:
- 레거시 문서(HWP/PDF/표/그림) 인식
- 평가 기준에 맞춘 실적보고서 구조 도출
- 점수 시뮬레이션 기반 콘텐츠 추천

**PoC 대상 평가항목**: "안전 보건"

**고객 제공 입력 자료(현재 보유)**:
1. 평가편람 및 실적보고서 작성지침
2. 한국전력기술 자사 실적보고서 + 관련 백데이터
3. A / B / C / D 등급의 타 기관 안전보건 실적보고서 + 관련 백데이터
   - 등급별 자료의 목적: **자사 보고서의 등급 시뮬레이션** (예: 현재 B예상 → A 상승 시나리오 추천)

**시장 규모 (도메인 확장 컨텍스트)**:
- 기획재정부 공공기관 경영평가 대상: 약 339개 기관 (공기업·준정부기관)
- 각 기관: 5~10명 × 1년 × 인건비 → 기관당 수억 원 인적 비용
- 등급(S/A/B/C/D/E)이 기관장 성과급·기관 명예에 직결

---

## 1. AX 정의 (Round 1, Q1)

**Question**: 'AX 플랫폼'의 핵심 정의

**Answer**: Agent eXperience — AI 에이전트를 위한 인프라
**Refinement (Anchor 컨텍스트 반영)**: **도메인 특화 Agentic AI 플랫폼** (1차 도메인: 공기업 경영평가)

---

## 2. 1차 타깃 사용자 (Round 1, Q2)

**Question**: 1차 타깃 사용자

**Answer (초기)**: 엔터프라이즈 개발자/아키텍트
**Refinement (Anchor 컨텍스트 반영)**:
- **실 사용자**: 공기업 경영평가팀 (비개발자, 도메인 전문가)
- **운영자**: 고객사 사내 IT 운영부서
- **구매 의사결정자**: 공기업 기획조정실 / 외주 SI 발주처

---

## 3. MVP 스코프 (Round 1, Q3)

**Question**: MVP 우선 제공 능력

**Answer (초기)**: 에이전트 런타임 + 오케스트레이션
**Refinement (Anchor 컨텍스트 반영)**:
- **Document Ingestion** (VLM 기반 HWP/PDF/표·그림 처리)
- **기준 매핑** (평가편람·작성지침 → RAG 인덱싱)
- **등급 시뮬레이션** (A/B/C/D 등급별 보고서 학습 기반 등급 예측)
- **보고서 초안 생성** (평가 기준 준수, 한국어 공문 스타일)
- **Recommendation 엔진** (Gap analysis 기반 점수 상승 콘텐츠 제안)

---

## 4. 차별화 핵심 (Round 1, Q4)

**Question**: 기존 솔루션(LangGraph, CrewAI, Copilot Studio 등) 대비 차별화

**Answer**: 한국어/한국 도메인 최적화
**Refinement**: **HWP 파싱 + 한국 공공 도메인 깊이 + 한국어 LLM 라우팅** (단일 LangGraph 경쟁이 아닌, 도메인 전문성으로 차별화)

---

## 5. 기술 스택 (Round 2, Q1)

**Question**: 기본 기술 스택

**Answer**: Go 컨트롤플레인 + TS 콘솔 + Python SDK
**Refinement (Anchor 컨텍스트 반영)**:
- **Python**: VLM/RAG/Document AI 파이프라인 — MVP 비중 최대
- **Go**: 컨트롤플레인 (에이전트 lifecycle, 워크플로 orchestrator, K8s operator)
- **TypeScript**: 콘솔 UI (React/Next.js), HWP/PDF 뷰어
- **HWP 파서**: 한컴 한글 변환기 통합 또는 hwp-converter (Python)

---

## 6. 배포 모델 (Round 2, Q2)

**Question**: 배포 모델

**Answer**: K8s 네이티브 + 셀프호스트 옵션
**근거**: 공공기관 망분리 환경, 데이터 주권 요구와 완전 정합

---

## 7. LLM/추론 전략 (Round 2, Q3)

**Question**: LLM/추론 프로바이더 전략

**Answer**: 자체 호스팅 오픈소스 우선 (vLLM + Llama/Qwen/EXAONE)
**Refinement (VLM 요구 추가)**:
- **VLM 후보**: Qwen2-VL (7B/72B) — 한국어 OCR 강점, vLLM 호환
- **텍스트 LLM 후보**: EXAONE 3.5 (LG AI Research, 한국어 우수), Llama 3.x, Qwen 2.5
- **이유**: 공공기관 망분리 + 데이터 주권 + 한국어 도메인 깊이의 3중 정합

---

## 8. GTM 1차 버티컬 (Round 2, Q4)

**Question**: 초기 GTM 버티컬

**Answer (초기)**: 금융·보험 (높은 ARR + 컴플라이언스)
**Refinement (Anchor 컨텍스트 반영)**: **공기업 경영평가 (Anchor: KEPCO E&C → 339개 공공기관)**
- 1차 12개월: 공기업 경영평가 솔루션 (KEPCO E&C 레퍼런스 확보)
- 2차 확장: ESG 보고서, 감사 보고서, 면허신청서, 정부 공문 자동화

---

## 9. 제품명 (Round 3, Q2)

**Answer**: iroum-ax (프로젝트명 그대로 제품명으로 사용)
- '이루다(이룸) + AX' 풀이의 한국 브랜드 에쿼티
- 도메인 후보: iroum-ax.com / iroum.ai
- SPEC ID prefix: `SPEC-AX-NNN`

---

## 10. 첫 SPEC 슬라이스 (Round 3, Q3)

**Answer (초기)**: 최소 런타임 코어
**Refinement (Anchor 컨텍스트 반영)**: **안전보건 PoC Walking Skeleton**
- 평가편람 1개 항목(안전보건) 파싱
- 한국전력기술 실적보고서 1개 파싱
- 평가 기준 → 자사 보고서 매핑 (RAG)
- 보고서 초안 1개 섹션 생성
- 1개 등급 시뮬레이션 (B vs A 차이 학습)
- CLI/API 우선, UI 후행

---

## 11. 품질 검수 강도 (Round 3, Q4)

**Answer**: thorough (plan-auditor + evaluator-active 교차 검증)
- 초기 SPEC은 이후 모든 구현의 골격이므로 적대적 감사 강도 최대

---

## 12. 비전 폭 (Round 4, Q1)

**Answer**: Anchor → Platform (솔루션 우선, 플랫폼 잠재력 보존)
- 1차 제품 = 공기업 경영평가 AI 솔루션 (KEPCO E&C anchor)
- 2년차 = 동일 아키텍처로 ESG / 감사 / 면허 등으로 확장 → 플랫폼화
- Linear/Notion이 검증한 startup 경로
