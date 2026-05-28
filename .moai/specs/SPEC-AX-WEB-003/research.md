# SPEC-AX-WEB-003 — Research

작성일: 2026-05-27
작성자: ircp
SPEC: SPEC-AX-WEB-003 (apps/web `next.config.ts` → `next.config.mjs` 전환으로 E2E 차단 해제)

---

## 1. 문제 진술 (Problem Statement)

SPEC-AX-E2E-001가 도입한 21건의 Playwright E2E 테스트(5 spec 파일)가 **`npm run dev` 기동 실패**로 전부 차단되었다. `npm run dev`는 Next.js 14.2.18 dev 서버를 띄우는 표준 경로지만, `apps/web/next.config.ts`가 TypeScript 전용 문법을 포함하면서 Next.js의 config 로더가 첫 줄(`import type ...`)에서 모듈 파싱에 실패한다.

본 SPEC은 **단일 root cause(설정 파일 확장자 + 1줄 type import + 1줄 type annotation)**를 최소 외과적으로 제거하여 dev 서버 기동을 복구하고, 21건 E2E 테스트가 모두 실행 가능한 상태로 되돌리는 것을 목적으로 한다.

---

## 2. Root Cause 분석

### 2.1 Next.js 14.2.18 config 파일 로더 제약 [verified]

Next.js 14.x는 config 파일을 `require()` 또는 native ESM `import()`로 직접 로드한다. 즉 Node.js가 실행 가능한 모듈 포맷만 허용:

- `next.config.js` (CommonJS)
- `next.config.mjs` (ES Module)
- `next.config.ts`는 별도 TypeScript 트랜스파일러(`ts-node` / `tsx` / `jiti`)가 dependency tree에 존재할 때에 한해 지원되며, Next.js 14.x는 이를 **autoload하지 않는다**.

Next.js 15+에서는 `next.config.ts` 네이티브 지원이 들어왔으나(turbopack 기반 transformer 사용), **본 프로젝트는 14.2.18에 핀**되어 있어 해당 경로가 닫혀 있다.

### 2.2 현재 `apps/web/next.config.ts` 진단 [verified, 44 lines read]

파일 첫 줄:
```typescript
import type { NextConfig } from "next";
```

이 `import type` 구문은 **TypeScript 컴파일 타임 전용 syntax**다. Node.js의 ESM/CJS 로더는 이를 인식하지 못하고 SyntaxError로 즉시 실패한다. 추가로 10번 라인:
```typescript
const nextConfig: NextConfig = {
```

의 `: NextConfig` type annotation도 TypeScript syntax — 동일한 이유로 Node 직접 로드 시 실패한다.

### 2.3 트랜스파일러 부재 검증 [verified via package.json read]

`apps/web/package.json`의 `dependencies` + `devDependencies` 전수 점검 결과:

- `ts-node` 부재
- `tsx` 부재
- `jiti` 부재 (Next.js 14가 .ts config를 로드할 때 사용할 유일한 자동 후보)
- `typescript: "5.4.5"` 존재하지만 이는 IDE/tsc 타입 검사용이며 Next.js config 로더가 자동으로 동원하지 않음

결론: 현재 의존성으로는 `next.config.ts`가 **결코 로드될 수 없는** 상태다.

### 2.4 차단 전파 경로

```
next.config.ts 파싱 실패
  → npm run dev 즉시 종료(non-zero exit)
    → apps/web/playwright.config.ts에 `webServer` 설정 부재(또는 webServer가 npm run dev에 의존)
      → Playwright runner가 baseURL에 GET → ECONNREFUSED
        → 21건 E2E 테스트 전부 실패/스킵
```

`playwright.config.ts`에 자동 `webServer`가 정의되어 있더라도 그 명령이 `npm run dev`이므로 동일하게 실패한다.

---

## 3. 영향 범위 인벤토리

### 3.1 E2E 테스트 목록 [verified via ls + grep]

| 파일 | `test()` 호출 수 |
|------|------------------|
| `apps/web/e2e/auth.spec.ts` | 3 |
| `apps/web/e2e/logout.spec.ts` | 2 |
| `apps/web/e2e/flow-analyst.spec.ts` | 4 |
| `apps/web/e2e/flow-admin.spec.ts` | 5 |
| `apps/web/e2e/rbac-viewer.spec.ts` | 7 |
| **합계** | **21** |

이 21건 전부가 본 SPEC의 회복 대상이다.

### 3.2 consumer-only 0-diff 경계 [HARD]

본 SPEC은 다음 디렉터리를 **수정하지 않는다**:

- `apps/control-plane/**` (Go control-plane, SPEC-AX-SERVER-001 외 7 SPECs의 산출물)
- `apps/pipeline/**` (Python pipelines, SPEC-AX-PIPE-001 / SPEC-AX-INGEST-001 / SPEC-AX-INTEG-001 산출물)
- `apps/web/src/**` (SPEC-AX-WEB-001 frontend 본체)
- `apps/web/e2e/**` (SPEC-AX-E2E-001 테스트 본체)
- `apps/web/playwright.config.ts`
- 루트 `package.json`, workspaces, `go.mod`, `pyproject.toml`

이 7개 frozen 영역 전부 git diff 기준 0-diff를 유지한다. orchestrator는 Phase A 완료 시 `git diff --quiet -- <frozen scope>`로 직접 검증한다.

### 3.3 수정 대상 파일 (총 1 파일 rename + 2줄 변경)

| 변경 | 파일 |
|------|------|
| rename | `apps/web/next.config.ts` → `apps/web/next.config.mjs` |
| 삭제 | Line 1: `import type { NextConfig } from "next";` |
| 변경 | Line 10: `const nextConfig: NextConfig = {` → `const nextConfig = {` |
| 보존 | 나머지 41 줄(주석 4 + 본문 37) 전부 그대로 유지 |

---

## 4. 해결 접근 (Fix Approach)

### 4.1 선택: `.mjs` 전환 (권장, 채택)

- **장점**: 변경량 최소(파일 1개 rename + 2줄 텍스트 편집), Next.js 14가 ESM 모듈로 직접 import → 트랜스파일러 의존성 0 추가, behavior 변화 0(설정 객체 형상 동일).
- **단점**: TypeScript 타입 안전성 상실(설정 객체 자체에 한정) — 단, `next.config`는 호출 빈도가 낮고 IDE가 JSDoc으로 `NextConfig` 타입을 여전히 제공할 수 있어 실 영향 미미.
- **위험도**: 매우 낮음. 설정 객체 구조 변경 없음 → rewrites/headers proxy 동작 동일.

### 4.2 대안 (기각)

| 대안 | 기각 사유 |
|------|----------|
| Next.js 15 업그레이드 | breaking changes(App Router 변경 / minimum React 19 / middleware 시그니처 변경) 대량 발생. 본 SPEC 범위 초과 → 별도 SPEC. |
| `jiti` / `tsx` devDependency 추가 + `.ts` 유지 | 패키지 1개 추가 + 부트 시간 증가 + 추후 transitive deps 관리 부담. 본 PoC 목적에 비해 과잉. |
| `.js`(CommonJS) 전환 | `.mjs`와 비교 시 미래 호환성 열위(현 Next.js 에코시스템은 ESM 우선). |

### 4.3 위험 평가

- **Build/lint regression**: `eslint-config-next` + `next lint`는 `next.config.*`를 lint 대상에서 제외. typecheck(`tsc --noEmit`)는 `apps/web/tsconfig.json`의 `include` 패턴에서 자동 제외(설정 파일은 보통 빌드 산출물). 영향 없음 예상 — Phase B에서 명시 검증.
- **Runtime regression**: 설정 객체 형상 100% 동일 → BFF rewrite + CSP headers 동작 보존.
- **CI regression**: 본 프로젝트는 CI 워크플로 미통합(SPEC-AX-E2E-001 §3 비목표). 영향 없음.

---

## 5. 검증 계획 (Verification Plan)

| Phase | 명령 | 합격 기준 |
|-------|------|-----------|
| Phase A | `git mv apps/web/next.config.ts apps/web/next.config.mjs` + 2줄 edit | git diff에 rename + 2줄 변경만 표시 |
| Phase B | `cd apps/web && npm run dev` (백그라운드) → `curl -sf http://localhost:3000/login` | dev 서버가 ready 출력 + `/login` 200 OK |
| Phase C | `cd apps/web && npm run test:e2e` | 21건 모두 pass (0 fail) |
| Frozen scope | `git diff --quiet -- apps/control-plane apps/pipeline apps/web/src apps/web/e2e apps/web/playwright.config.ts go.mod pyproject.toml package.json` | exit code 0 (zero diff) |

---

## 6. Open Questions (해결 완료 → §6 비어 있음)

본 SPEC은 단일 외과적 변경이므로 OPEN 질의 없음. 모든 가정은 위 §1~§5에서 검증 완료.

---

## 7. 참고 (References)

- `apps/web/package.json`: Next.js 14.2.18 dependency, no ts-node/tsx/jiti.
- `apps/web/next.config.ts:1` (`import type { NextConfig } from "next";`) — 차단 원인.
- `apps/web/next.config.ts:10` (`const nextConfig: NextConfig = {`) — 차단 원인 2.
- `apps/web/e2e/*.spec.ts` — 21건 차단된 테스트(5 파일).
- SPEC-AX-E2E-001 §1.3 — SUT 경로 카탈로그(본 SPEC의 회복 대상 표면).
- Next.js Discussion (vercel/next.js #51483): TS config support는 Next.js 15부터 네이티브화, 14.x는 트랜스파일러 의존.
