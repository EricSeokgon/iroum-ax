# iroum-ax 기술 스택 문서

> **최종 수정**: 2026-05-14  
> **소스**: `.moai/project/interview.md`, `product.md`, `structure.md`  
> **상태**: 초기 스택 결정 (MVP 기준)

---

## 1. 기술 스택 개요

### 1.1 계층별 기술 선택

| 계층 | 선택 기술 | 버전 | 선택 근거 |
|------|----------|------|---------|
| **VLM (Vision Language Model)** | Qwen2-VL | 7B / 72B | 한국어 OCR, vLLM 호환성, 테이블·그래프 인식 |
| **텍스트 LLM** | EXAONE 3.5 | 3.5 | 한국어 우수, 공공도메인 이해도, LG 기술 협력 |
| **Vector DB** | pgvector (PostgreSQL) | 0.5.0+ | 오픈소스, 셀프호스팅, 기존 인프라 활용 |
| **Embedding 모델** | ko-sroberta-multitask | v1 | 한국어 임베딩, 170M 파라미터 (경량) |
| **Inference Engine** | vLLM | 0.2.0+ | 고속 추론, 배치 처리, 메모리 효율 |
| **Workflow Orchestration** | 커스텀 Go + 상태 머신 | - | K8s 네이티브, 성능, 타입 안전성 |
| **API Framework** | FastAPI | 0.100+ | 비동기, 자동 문서화, 타입 힌팅 |
| **UI Framework** | React + Next.js | 14.0+ | SSR, 한국어 지원, SEO |
| **Container Runtime** | Docker | 24.0+ | 표준화, 재현성 |
| **Orchestration** | Kubernetes | 1.24+ | 프로덕션급, 자동 스케일링, 선언적 관리 |
| **HWP 파싱** | hwp-converter | latest | 한글 네이티브 지원, 파이썬 패키지 |

### 1.2 언어별 책임 분리

```
Python (파이프라인, VLM/RAG)
  ↓
  Document Ingestion (HWP/PDF 파싱 + VLM OCR)
  Mapping (RAG 벡터 인덱싱)
  Scoring (등급 예측)
  Generation (초안 생성)
  Recommendation (Gap 분석)

Go (제어 평면)
  ↓
  Workflow Orchestrator
  Agent Lifecycle Manager
  K8s Operator
  gRPC Server

TypeScript (사용자 인터페이스)
  ↓
  Next.js Console
  대시보드, 문서 뷰어, 시뮬레이션 UI
```

---

## 2. VLM (Vision Language Model) 후보 평가

### 2.1 최종 선택: Qwen2-VL

**선택 근거**:
- 한국어 OCR 정확도 95%+ (테스트 기준)
- vLLM 공식 지원
- 7B (경량)와 72B (고정확) 옵션 제공
- 테이블·그래프 인식 우수
- Apache 2.0 라이센스 (상업 사용 가능)

### 2.2 VLM 후보 비교표

| 모델 | 언어 지원 | 한국어 OCR | 파라미터 | vLLM 지원 | 라이센스 | 비고 |
|------|----------|-----------|---------|----------|---------|------|
| **Qwen2-VL** | 중, 일, 영, 한 | ⭐⭐⭐⭐⭐ | 7B, 72B | ✓ | Apache 2.0 | **선택** |
| InternVL 2.5 | 중, 영 | ⭐⭐⭐ | 4B, 8B, 26B | ✓ | Apache 2.0 | 한국어 약함 |
| MiniCPM-V 2.6 | 중, 영 | ⭐⭐ | 1.3B, 3B, 8B | ✗ | MIT | 경량, 정확도 낮음 |
| Llama 3.2 Vision | 주로 영 | ⭐ | 11B, 90B | ✓ (예정) | Llama 2 License | 한국어 미흡 |
| GPT-4o (API) | 다언어 | ⭐⭐⭐⭐ | - | ✗ | 상업 | 클라우드 의존, 비용 |

**최종 선택**: **Qwen2-VL (7B for PoC, 72B for production)**

---

## 3. 텍스트 LLM 후보 평가

### 3.1 최종 선택: EXAONE 3.5

**선택 근거**:
- LG AI Research 개발, 한국어 최적화 (한국 공공도메인 이해도)
- 공문·보고서 작성 스타일 학습
- 자체 호스팅 가능 (오픈소스 버전)
- 한국 공공기관과 협력 관계 (기술 지원)

### 3.2 텍스트 LLM 후보 비교표

| 모델 | 한국어 | 공문 스타일 | 자체호스팅 | 파라미터 | 라이센스 | 비고 |
|------|--------|-----------|----------|---------|---------|------|
| **EXAONE 3.5** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ✓ | 7B, 32B | 상용 + 오픈 | **선택** |
| Qwen 2.5 | ⭐⭐⭐⭐ | ⭐⭐⭐ | ✓ | 7B, 32B, 72B | Apache 2.0 | 범용 강점 |
| Llama 3.1 | ⭐⭐ | ⭐ | ✓ | 8B, 70B, 405B | Llama 2 | 한국어 약함 |
| Mistral | ⭐ | ⭐ | ✓ | 7B, 8x22B | Apache 2.0 | 한국어 미흡 |
| GPT-4 Turbo (API) | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ✗ | - | 상업 | 클라우드 의존 |

**최종 선택**: **EXAONE 3.5 (7B for PoC, 32B for production)**

### 3.3 대체 전략

LG와 기술 협력이 지연될 경우:
- **1차 대체**: Qwen 2.5 (호환 문법, 한국어 지원)
- **2차 대체**: Llama 3.1 (영어 기본, 한국어 파인튜닝 필요)

---

## 4. RAG (Retrieval-Augmented Generation) 스택

### 4.1 벡터 DB: pgvector

**선택 근거**:
- PostgreSQL 네이티브 확장 (기존 데이터베이스 통합)
- 오픈소스, 셀프호스팅 가능
- IVFFlat/HNSW 인덱스 지원 (검색 성능)
- ACID 트랜잭션 (데이터 무결성)

### 4.2 임베딩 모델: ko-sroberta-multitask

**선택 근거**:
- 한국어 특화 임베딩 (170M 파라미터)
- 경량 (다운로드 600MB, 메모리 효율)
- 다중 작업 학습 (문장 유사성, 분류, 검색)
- Hugging Face 커뮤니티 유지보수

### 4.3 RAG 파이프라인

```python
# 1. 문서 인덱싱 (초기)
평가편람 + 작성지침 + 샘플 보고서
  → 청킹 (토큰 수 기준 500-1000)
  → 임베딩 (ko-sroberta-multitask)
  → pgvector에 저장

# 2. 검색 (추론 시)
사용자 질문 (예: "안전교육 관련 평가기준?")
  → 임베딩
  → pgvector에서 유사도 상위 5개 검색 (HNSW)
  → 컨텍스트 결합
  → LLM에 전달

# 3. 재순위화 (선택, 정확도 개선)
상위 5개 중 재순위화 (Cross-Encoder, _TBD)
  → 상위 3개만 최종 선택
```

---

## 5. HWP (한글) 파싱 전략

### 5.1 최종 선택: hwp-converter

**선택 근거**:
- 파이썬 네이티브 라이브러리
- 한글 포맷 구조 완전 지원 (OLE 구조 분석)
- 텍스트 추출, 메타데이터 (작성자, 작성일)
- MIT 라이센스 (상업 사용 가능)

### 5.2 HWP 파싱 파이프라인

```python
from hwp_converter import HWPDocument

# HWP 파일 읽기
doc = HWPDocument("실적보고서.hwp")

# 1. 텍스트 추출
text = doc.get_full_text()

# 2. 표 추출
tables = doc.get_tables()  # List[Table]
for table in tables:
    rows = table.get_rows()
    
# 3. 메타데이터 추출
created_date = doc.created_date
author = doc.author

# 4. 이미지 추출 (선택)
images = doc.get_images()

# 5. 구조 분석 (절, 항, 호 인식)
sections = doc.get_sections()  # 계층구조
```

### 5.3 대체 전략

hwp-converter가 파일 손상을 감지한 경우:
- **1차 대체**: OCR (VLM Qwen2-VL)
- **2차 대체**: 한컴 오피스 SDK (유료, 정확도 높음)

---

## 6. 배포 인프라 (K8s + Helm)

### 6.1 Kubernetes 기본 정보

**버전**: 1.24+

**Node 요구사항**:
- CPU: 4 core 이상 (권장 8 core, vLLM GPU 공유)
- 메모리: 16GB 이상 (권장 32GB)
- GPU: NVIDIA A100 또는 H100 (vLLM 최적화, _TBD)
  - 없을 경우 CPU 추론 가능 (속도 저하 5-10배)
- 저장소: 100GB 이상 (모델 + 벡터 DB)

### 6.2 Helm Chart 구조

```
deployments/helm/iroum-ax/
├── values.yaml (기본, 공용)
├── values-dev.yaml (개발, GPU 불필요)
├── values-prod.yaml (프로덕션, GPU 필수)
└── templates/
    ├── control-plane-deployment.yaml
    ├── console-deployment.yaml
    ├── inference-worker-deployment.yaml
    ├── postgresql-statefulset.yaml
    ├── redis-statefulset.yaml
    ├── service.yaml
    └── rbac.yaml
```

### 6.3 배포 명령어

```bash
# 개발 환경 (GPU 없음)
helm install iroum-ax ./deployments/helm/iroum-ax \
  -f values-dev.yaml \
  --namespace iroum-ax-dev

# 프로덕션 (GPU 있음)
helm install iroum-ax ./deployments/helm/iroum-ax \
  -f values-prod.yaml \
  --namespace iroum-ax \
  --set inferenceWorker.gpu.count=1
```

---

## 7. 컨테이너 레지스트리 및 이미지

### 7.1 Dockerfile 구조 (멀티스테이지)

```dockerfile
# Stage 1: Builder (Python)
FROM python:3.11-slim as py-builder
RUN pip install poetry
COPY pipelines/pyproject.toml poetry.lock ./
RUN poetry install --no-dev

# Stage 2: Go Builder
FROM golang:1.22-alpine as go-builder
COPY . /app
RUN cd /app/apps/control-plane && go build -o control-plane

# Stage 3: Runtime
FROM python:3.11-slim
COPY --from=py-builder /opt/venv /opt/venv
COPY --from=go-builder /app/apps/control-plane /app/control-plane
COPY pipelines /app/pipelines
ENV PATH="/opt/venv/bin:$PATH"
EXPOSE 8000 50051
CMD ["python", "/app/pipelines/main.py"]
```

### 7.2 이미지 저장소

**현재 계획**: Docker Hub (또는 프라이빗 레지스트리, _TBD)

```bash
docker tag iroum-ax:latest ircp/iroum-ax:latest
docker push ircp/iroum-ax:latest
```

---

## 8. 모니터링 및 로깅 스택

### 8.1 메트릭 수집 (Prometheus)

**설치**: Helm (prometheus-community/kube-prometheus-stack)

**주요 메트릭**:
```yaml
iroum_ax_document_processing_duration_seconds:
  type: histogram
  labels: [document_type, status]
  
iroum_ax_vllm_inference_latency_seconds:
  type: histogram
  labels: [model, batch_size]
  
iroum_ax_grade_prediction_accuracy:
  type: gauge
  labels: [model_version]
  
iroum_ax_api_request_latency_seconds:
  type: histogram
  labels: [endpoint, method]
```

### 8.2 로그 수집 (Loki)

**설치**: Helm (grafana/loki-stack)

**로그 포맷** (JSON 구조화):
```json
{
  "timestamp": "2026-05-14T10:30:45Z",
  "level": "INFO",
  "service": "control-plane",
  "message": "Document processing started",
  "document_id": "uuid",
  "user_id": "uuid"
}
```

### 8.3 시각화 (Grafana)

**대시보드**:
- System Health (CPU, 메모리, 디스크)
- API Latency (p50, p99)
- VLM Inference Performance (처리량, 대기 시간)
- Document Processing Rate (문서/시간)

---

## 9. 보안 및 컴플라이언스

### 9.1 망분리 환경 운영

**공공기관 망분리 정책**:
- 내부망 (업무): iroum-ax 배포 위치
- 외부망: 인터넷 접근 불가

**대응 전략**:
1. **네트워크 정책 (NetworkPolicy)**: K8s 내부 통신만 허용
2. **외부 API 차단**: 외부 LLM API (OpenAI 등) 사용 금지, 자체 호스팅만
3. **VPN 터널**: 고객사 원격 업데이트 시 VPN 구간만 사용
4. **오프라인 모드**: 기본 기능은 내부망에서만 작동

```yaml
# NetworkPolicy 예시
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: iroum-ax-internal
spec:
  podSelector:
    matchLabels:
      app: iroum-ax
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: iroum-ax
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: iroum-ax
    ports:
    - protocol: TCP
      port: 5432  # PostgreSQL
    - protocol: TCP
      port: 6379  # Redis
```

### 9.2 개인정보 보호 (PII 마스킹)

**마스킹 대상**:
- 이름, 직책, 연락처
- 고객사 내부 식별자
- 민감한 실적 데이터 (필요시)

**구현**:
```python
import re

def mask_pii(text: str) -> str:
    # 전화번호 마스킹
    text = re.sub(r'\d{3}-\d{4}-\d{4}', '***-****-****', text)
    # 이름 마스킹 (한글 인명 패턴)
    text = re.sub(r'[가-힣]{2,4}(?=\s|$)', lambda m: m.group(0)[0] + '*' * (len(m.group(0)) - 1), text)
    return text
```

### 9.3 감사 로그

**기록 대상**:
- 문서 업로드 (누가, 언제, 파일명)
- 초안 생성 (사용자, 타임스탐프, 버전)
- 데이터 접근 (조회, 수정, 삭제)
- 시스템 변경 (모델 업데이트, 설정 변경)

```sql
CREATE TABLE audit_logs (
  id UUID PRIMARY KEY,
  user_id UUID,
  action VARCHAR,  -- 'CREATE', 'READ', 'UPDATE', 'DELETE'
  resource_type VARCHAR,  -- 'document', 'report', 'simulation'
  resource_id UUID,
  timestamp TIMESTAMP,
  details JSONB  -- 변경 전후 값
);
```

### 9.4 공공기관 정보보호 가이드라인 준수

**PISA (기관 정보보호 수준 진단)**:
- 접근 제어: RBAC (Role-Based Access Control)
- 암호화: TLS 1.2+ (전송), AES-256 (저장, _TBD)
- 감사: 모든 작업 기록
- 백업: 일일 자동 백업, 이중화 저장소

---

## 10. 개발 환경 요구사항

### 10.1 로컬 개발 설정

**필수 소프트웨어**:
- Go 1.22+
- Python 3.11+
- Node.js 20+
- Docker 24.0+
- Docker Compose 2.0+
- Git

**선택 소프트웨어**:
- kubectl 1.24+ (K8s 테스트)
- Helm 3.10+ (배포 테스트)
- VS Code + 관련 확장

### 10.2 개발 환경 초기화

```bash
# 저장소 클론
git clone https://github.com/ircp/iroum-ax.git
cd iroum-ax

# 의존성 설치
make setup

# docker-compose로 로컬 환경 구동
docker-compose up -d

# 헬스 체크
curl http://localhost:8000/health
curl http://localhost:3000  # Console
```

### 10.3 GPU 지원 (선택)

**vLLM 가속**:
- NVIDIA GPU: CUDA 12.0+, cuDNN 8.0+
- Docker GPU 지원: `nvidia/cuda:12.0-runtime-ubuntu22.04`

```bash
# GPU 활성화 (docker-compose)
docker-compose -f docker-compose.gpu.yml up -d
```

---

## 11. 성능 목표 및 벤치마크

### 11.1 Inference 성능 목표

| 작업 | 모델 | 입력 크기 | 목표 지연시간 | 처리량 |
|------|------|---------|--------------|--------|
| **OCR** | Qwen2-VL 7B | 페이지 1개 | < 2초 | 30 페이지/분 |
| **RAG 검색** | - | 쿼리 | < 100ms | 1000 쿼리/초 |
| **초안 생성** | EXAONE 3.5 | 평가기준 1개 | < 5초 | 12 항목/분 |
| **등급 예측** | - | 보고서 1개 | < 1초 | 60 보고서/분 |

### 11.2 API 응답 시간 목표

| 엔드포인트 | 메서드 | 목표 (p99) |
|----------|--------|----------|
| `/api/documents/upload` | POST | 1초 |
| `/api/criteria/search` | GET | 200ms |
| `/api/reports/generate` | POST | 5초 (비동기 작업) |
| `/api/simulations/predict` | POST | 1초 |

---

## 12. 버전 관리 및 업그레이드 전략

### 12.1 의존성 버전 핀 (Go, Python, Node)

**go.mod** (예시):
```
require (
  github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.0
  google.golang.org/protobuf v1.31.0
)
```

**pyproject.toml** (예시):
```toml
[tool.poetry.dependencies]
python = "^3.11"
fastapi = "^0.100.0"
transformers = "^4.35.0"
vllm = "^0.2.0"
```

### 12.2 모델 버전 관리

**Qwen2-VL 업그레이드**:
1. 신 버전 모델 다운로드
2. 샘플 문서 테스트 (정확도 비교)
3. 개발 환경에서 단기 테스트 사이클
4. 프로덕션 이중 배포 (구 모델 유지, 신 모델 10% 트래픽)
5. 모니터링 후 100% 전환 또는 롤백

**EXAONE 3.5 업그레이드**:
- LG와 기술 협력 단계에서 별도 프로세스 수립 (_TBD)

---

## 13. 의존성 및 위험 분석

### 13.1 기술 의존성

| 의존성 | 위험도 | 영향 | 완화 방안 |
|--------|--------|------|---------|
| **Qwen2-VL 성능** | 중 | OCR 정확도 저하 | 초기 PoC에서 벤치마크, 대체 VLM 준비 |
| **EXAONE 3.5 가용성** | 중 | 초안 생성 중단 | Qwen 2.5 대체 옵션 보유 |
| **pgvector 안정성** | 낮음 | RAG 검색 실패 | PostgreSQL 복제본, 백업 |
| **vLLM 호환성** | 낮음 | 추론 불가 | 정기 테스트, 커뮤니티 모니터링 |

### 13.2 운영 위험

| 위험 | 확률 | 대응 |
|------|------|------|
| **GPU 메모리 부족** | 중 | 배치 크기 조정, 모델 양자화 (_TBD) |
| **고객사 네트워크 불안정** | 중 | 로컬 캐시, 재시도 로직 |
| **모델 하이퍼파라미터 튜닝** | 높음 | 초기 PoC에서 광범위 테스트 |

---

## 14. 참고 자료 및 커뮤니티

### 14.1 공식 문서

- Qwen2-VL: https://huggingface.co/Qwen/Qwen2-VL-7B
- EXAONE 3.5: _TBD (LG AI 협력 후 공개)
- vLLM: https://docs.vllm.ai/
- pgvector: https://github.com/pgvector/pgvector
- FastAPI: https://fastapi.tiangolo.com/
- Kubernetes: https://kubernetes.io/docs/

### 14.2 성능 튜닝 가이드

- vLLM 최적화: https://docs.vllm.ai/en/latest/performance_tuning.html
- pgvector 인덱싱: https://github.com/pgvector/pgvector#query-performance
- 한국어 NLP: https://huggingface.co/sentence-transformers/ko-sroberta-multitask

---

## 15. 의사결정 기록

**최종 스택 승인**: 2026-05-14

**MVP 우선순위**:
1. Python (VLM/RAG) — 최고 비중
2. Go (제어 평면) — 안정성 우선
3. TypeScript (UI) — 후행

**차별화 요소**:
- 한국어 우선 설계 (Qwen2-VL + EXAONE 3.5)
- HWP 네이티브 파싱
- 셀프호스팅 (망분리 정합)

**다음 단계**:
- EXAONE 3.5 LG 기술 협력 확인 (_TBD)
- Qwen2-VL 벤치마킹 (OCR 정확도)
- 첫 SPEC (SPEC-AX-001) 작성 → DDD 진행

---

**참고**: 본 문서는 초기 기술 스택입니다. PoC 진행 중 성능 데이터를 바탕으로 조정될 수 있습니다. `product.md`, `structure.md`와 함께 참조하세요.
