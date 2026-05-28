# SPEC-AX-WEB-003 OPEN-AC

작성일: 2026-05-27
관련 AC: AC-E2E-001 (REQ-WEB-E2E-001 D1 fallback)

---

## 차단 사유

Go control-plane(:8080) 미가동으로 7개 E2E 시나리오가 backend live 의존 실패.
`page.route` mocking으로 회피 불가한 시나리오: JWT 쿠키 검증 미들웨어 / in-page RBAC 차단 렌더링.

## E2E 실행 결과 (2026-05-27)

- 총 21건: **9 passed / 7 failed / 5 skipped**
- 차단 전 상태: 0건 실행 (next.config.ts parse error로 npm run dev 즉시 종료)
- 차단 해소 후: 9건 PASS, 실행 가능 상태로 전환

## 7건 backend-live 실패 목록

| spec | test name | 의존 endpoint |
|------|-----------|--------------|
| e2e/auth.spec.ts:56 | AC-AUTH-003 — 만료 JWT 쿠키 시 /login 리다이렉트 | JWT 검증 미들웨어 (Go auth) |
| e2e/flow-admin.spec.ts:20 | AC-VIS-ADMIN-001 — admin 감사 로그 페이지 접근 | GET /api/v1/audit-logs |
| e2e/flow-admin.spec.ts:36 | AC-VIS-ADMIN-002 — admin 루브릭 페이지 접근 | GET /api/v1/rubric |
| e2e/flow-analyst.spec.ts:22 | AC-VIS-ANALYST-001 — analyst 증빙 페이지 업로드 버튼 | GET /api/v1/evidence |
| e2e/flow-analyst.spec.ts:153 | AC-FLOW-ANALYST-DENY-001 — analyst audit-logs in-page 차단 | RBAC 미들웨어 |
| e2e/rbac-viewer.spec.ts:95 | AC-RBAC-DENY-001 — viewer audit-logs in-page 차단 메시지 | RBAC 미들웨어 |
| e2e/rbac-viewer.spec.ts:112 | AC-RBAC-DENY-002 — viewer rubric in-page 차단 메시지 | RBAC 미들웨어 |

## 해소 조건

Go control-plane 가동 환경(로컬 또는 CI staging)에서 재실행 시 21/21 PASS 예상.
해소 후 이 파일 삭제 및 AC-E2E-001 완전 PASS 기록.
