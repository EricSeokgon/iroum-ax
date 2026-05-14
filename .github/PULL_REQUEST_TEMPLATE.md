<!-- PR 제출 전 다음 체크리스트를 확인해주세요 -->

## 변경 사항 요약

<!-- 이 PR이 무엇을 변경하는지 1-3 문장으로 설명 -->

## 관련 SPEC / Issue

<!-- 해당되는 항목 작성 -->

- SPEC: SPEC-AX-XXX-NNN
- Issue: #N
- 후속 작업 SPEC: SPEC-AX-XXX-NNN

## 변경 유형

- [ ] 새 기능 (`feat`)
- [ ] 버그 수정 (`fix`)
- [ ] 리팩토링 (`refactor`)
- [ ] 문서 (`docs`)
- [ ] 테스트 (`test`)
- [ ] 빌드/CI (`build`/`ci`)
- [ ] 성능 (`perf`)
- [ ] 보안 (`security`)
- [ ] 기타 (`chore`)

## 영향 범위

<!-- 변경이 영향을 미치는 영역 체크 -->

- [ ] Python pipelines (`pipelines/`)
- [ ] Go control plane (`apps/control-plane/`)
- [ ] 공유 패키지 (`pkg/`)
- [ ] 스키마 (`schemas/proto`, `schemas/openapi`)
- [ ] 인프라 (`Dockerfile`, `deployments/`)
- [ ] CI/CD (`.github/workflows/`)
- [ ] 프로젝트 문서 (`.moai/project/`)
- [ ] SPEC 문서 (`.moai/specs/SPEC-AX-XXX-NNN/`)

## TRUST 5 체크리스트

<!-- 변경 사항이 TRUST 5 원칙을 준수하는지 확인 -->

- [ ] **Tested**: 신규/변경 코드에 대한 테스트 추가 또는 보강 (커버리지 ≥ 85% 권장)
- [ ] **Readable**: 영어 식별자 + 한국어 주석 (per `language.yaml` `code_comments: ko`)
- [ ] **Unified**: `ruff check` (Python) / `golangci-lint run` (Go) → 0 errors
- [ ] **Secured**: OWASP 가이드 준수, 시크릿 하드코딩 없음, 외부 서비스 호출 없음 (망분리 정합)
- [ ] **Trackable**: Conventional commits, `@MX:ANCHOR/NOTE/WARN/TODO` 태그 적절 사용

## 테스트 결과

<!-- 로컬에서 실행한 테스트 결과 -->

```
# Python
$ pytest tests/unit/ -m "not integration and not gpu" --cov
...

# Go 단위
$ go test ./apps/control-plane/internal/...
...

# Go 통합 (선택)
$ go test -tags=integration ./apps/control-plane/internal/...
...
```

## 검토 요청 영역

<!-- 리뷰어가 특히 주의 깊게 봐야 할 부분 -->

## Breaking Change 여부

- [ ] 예 (아래에 마이그레이션 가이드 작성)
- [ ] 아니오

<!-- 예일 경우:
### Breaking Change 설명
- 이전 동작: ...
- 새 동작: ...
- 마이그레이션: ...
-->

## 후속 작업

<!-- 이 PR 이후 필요한 추가 작업 -->

- [ ] 문서 업데이트 (`.moai/project/codemaps/`)
- [ ] CHANGELOG 갱신
- [ ] 후속 SPEC 작성 (SPEC-AX-XXX-NNN)
- [ ] 운영 환경 설정 변경 (`.env.example`, Helm values)

---

<!-- 자동 생성된 PR 본문 검토 후 불필요한 섹션은 삭제해주세요 -->
