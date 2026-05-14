---
name: 버그 리포트
about: 동작 오류 또는 예기치 못한 결과를 신고합니다
title: "[BUG] "
labels: ["bug", "triage"]
assignees: []
---

## 버그 요약

<!-- 한 문장으로 무엇이 잘못되었는지 설명 -->

## 재현 절차

1. ...
2. ...
3. ...

## 예상 동작

<!-- 무엇이 일어났어야 하는지 -->

## 실제 동작

<!-- 무엇이 실제로 일어났는지 -->

## 영향 범위

- [ ] Python pipelines (REQ-AX-001~005, REQ-UBI)
- [ ] Go control plane (REQ-CTRL-001~005, REQ-CTRL-UBI)
- [ ] 인프라 (Docker, K8s, Postgres, Redis)
- [ ] CI/CD
- [ ] 기타: ___

## 환경

- OS: <!-- e.g., Ubuntu 22.04, macOS 14, Windows 11 -->
- Python 버전: <!-- e.g., 3.11.5 -->
- Go 버전: <!-- e.g., 1.25.0 -->
- 관련 의존성 버전:
- 배포 환경: <!-- local / Docker / K8s / 기타 -->

## 로그 / 에러 메시지

<details>
<summary>로그 펼치기</summary>

```
(여기에 관련 로그 붙여넣기)
```

</details>

## 재현 가능성

- [ ] 항상 재현됨
- [ ] 가끔 재현됨 (X% 비율)
- [ ] 특정 조건에서만 재현됨 (조건: ___)
- [ ] 1회만 발생 (재현 불가)

## 추가 컨텍스트

<!-- 관련 SPEC, PR, 다른 issue 링크 -->
