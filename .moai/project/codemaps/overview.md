# iroum-ax 아키텍처 개요 (Placeholder)

> 이 문서는 신규 프로젝트 플레이스홀더입니다. 코드베이스 구축 후 `/moai codemaps` 또는 `/moai project codemaps` 명령으로 자동 생성됩니다.
> 현재 단계: Discovery 완료 → 프로젝트 문서 생성 완료 → SPEC-AX-001 기획 대기

---

## 프로젝트 목표 (요약)

**iroum-ax**는 공기업 경영평가 보고서 작성과 점수 시뮬레이션을 자동화하는 도메인 특화 Agentic AI 플랫폼입니다.

### Anchor → Platform 전략

- **1차(Anchor)**: 공기업 경영평가 솔루션 (KEPCO E&C 안전보건 PoC)
- **2차(확장)**: ESG 보고서, 감사 보고서, 면허신청서, 정부 공문 자동화
- **장기(Platform)**: 도메인 특화 AX 플랫폼 (Korea Sovereign Agentic AI)

### 핵심 차별화

- 한국어 + 한국 공공 도메인 깊이
- HWP/PDF 파싱 + 한국 공기업 망분리 운영
- 자체 호스팅 한국어 LLM (Qwen2-VL + EXAONE)
- 정성 평가 → 등급 시뮬레이션 + Gap-driven Recommendation

---

## 향후 생성될 Codemaps 파일

코드 구현이 시작되면 다음 5개 파일이 본 디렉토리에 자동 생성됩니다:

| 파일 | 내용 |
|---|---|
| `overview.md` | 본 파일 — 고수준 아키텍처 요약, 시스템 경계 (구현 후 자동 갱신) |
| `modules.md` | 모듈별 책임/공개 인터페이스/내부 의존성 |
| `dependencies.md` | 외부 패키지 그래프, 모듈 간 의존 방향 |
| `entry-points.md` | CLI 명령, API 라우트, 이벤트 핸들러, K8s Operator 진입점 |
| `data-flow.md` | 문서 인입 → 파싱 → RAG → 보고서 생성 흐름, 상태 관리 |

---

## 참조 문서

- 제품 비전: `.moai/project/product.md`
- 모노레포 구조 계획: `.moai/project/structure.md`
- 기술 스택 결정: `.moai/project/tech.md`
- Discovery 인터뷰 원본: `.moai/project/interview.md`

---

> 이 플레이스홀더는 첫 코드가 작성된 직후 `/moai codemaps` 실행으로 자동 대체됩니다.
