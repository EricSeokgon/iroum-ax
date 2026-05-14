# 아키텍처 코드맵 (codemaps/)

iroum-ax 프로젝트의 모노레포 구조와 모듈 간 관계를 시각화한 문서 집합입니다.

## 문서 구성

### 1. [overview.md](./overview.md) — 전체 시스템 아키텍처
- 모노레포 레이어 (Python 파이프라인 | Go 제어 평면 | TypeScript 콘솔)
- 데이터 흐름 (HWP 입력 → Recommendation 출력 E2E)
- 계층 간 통신 규약 (gRPC/REST/FastAPI)

### 2. [pipelines.md](./pipelines.md) — Python 파이프라인 모듈 맵
- 5개 도메인 (ingestion | mapping | scoring | generation | recommendation)
- 17개 핵심 모듈의 책임과 진입점
- REQ-AX 매핑 (각 모듈이 어떤 요구사항을 구현하는지)
- 모듈 간 의존성 (fan_in/fan_out)
- @MX:ANCHOR 위치 (high fan-in 함수)

### 3. [pkg.md](./pkg.md) — 공유 라이브러리 맵
- 5개 Pydantic 데이터 모델 (Document | Criterion | Report | Simulation | Recommendation)
- 에러 정의 (HWPParseError, RAGInsufficientContextError 등)
- 로깅 구조

### 4. [data-flow.md](./data-flow.md) — Walking Skeleton E2E 데이터 흐름
- 단계별 데이터 변환 (HWP → 파싱 → 임베딩 → 예측 → 초안 → 추천)
- 각 단계 입출력 형식
- PostgreSQL + Redis 데이터 저장 경로

### 5. [req-traceability.md](./req-traceability.md) — 요구사항 추적성 매트릭스
- 24개 AC (수락 기준)를 구현 파일 및 테스트 파일과 연결
- REQ-UBI (데이터 주권, 언어, 감사 로깅) 및 REQ-AX-001~005 매핑
- 테스트 커버리지 (AC당 테스트 케이스)

## 사용 방법

### 시스템 이해하기
1. **처음 시작**: overview.md → pipelines.md 순서로 읽기
2. **특정 모듈 파악**: pipelines.md 에서 모듈 찾기 → 해당 파일 경로 확인

### 구현 시 참조
1. **요구사항 구현**: req-traceability.md 에서 AC 찾기 → 해당 모듈/테스트 파일 확인
2. **모듈 간 통신**: data-flow.md 에서 데이터 경로 확인 → pkg.md 에서 메시지 포맷 검증

### 의존성 관리
1. **모듈 변경 영향도**: pipelines.md 의 fan_in/fan_out 확인
2. **@MX:ANCHOR 함수 수정**: 영향받는 caller 목록 확인 후 변경

## 용어정리

| 용어 | 정의 |
|------|------|
| **REQ-UBI** | 시스템 전반 불변 조건 (데이터 주권, 언어, 감사) |
| **REQ-AX-NNN** | 특정 기능 요구사항 (NNN = 001~005 = 5개 MVP 기능) |
| **AC (Acceptance Criteria)** | 수락 기준: 각 요구사항을 확인하는 구체적 시나리오 |
| **fan_in** | 함수를 호출하는 caller 개수 (fan_in >= 3 → @MX:ANCHOR 필수) |
| **fan_out** | 함수가 호출하는 피호출자 개수 |
| **@MX:ANCHOR** | 고도로 재사용되는 함수 (변경 시 주의 필요) |

## 최종 업데이트

- **작성일**: 2026-05-14
- **소스**: SPEC-AX-001 v0.1.2 (Walking Skeleton Complete)
- **커버리지**: 모든 17개 파이프라인 모듈 + 5개 공유 모델 포함
