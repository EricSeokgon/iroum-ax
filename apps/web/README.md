# @iroum-ax/web — Phase A

SPEC-AX-WEB-001 Phase A 산출물. Next.js 14 App Router 기반 PoC 데모 웹 대시보드의 부트스트랩 + BFF 인증 베이스.

## 범위

본 워크스페이스(Phase A)는 다음을 제공합니다:

- Next.js 14 + TypeScript + Tailwind CSS + shadcn/ui 부트스트랩
- HttpOnly 쿠키 기반 BFF 인증 (Keycloak OIDC + PKCE)
- `/api/auth/login`, `/api/auth/callback`, `/api/auth/refresh`, `/api/auth/logout`, `/api/auth/me`
- middleware 기반 `/dashboard/*` 1차 보호 + RSC 2차 방어
- 좌측 네비게이션 + 우상단 사용자 메뉴 (역할 기반 가시성)
- 로그인 화면 (Keycloak SSO redirect)

Phase B 이후 범위(증빙·점수·리포트·리뷰·감사 로그·루브릭)는 본 PR에 포함되지 않습니다.

## 실행 (로컬)

전제: Node.js 20 LTS+, npm 10+, Keycloak `iroum-ax` realm + `iroum-ax-web` client 활성, Go control-plane `:8080` 실행 중.

1. 루트에서 의존성 설치
   ```bash
   npm install --workspace=@iroum-ax/web
   ```
2. 환경 변수 설정 — `apps/web/.env.example`을 `.env.local`로 복사 후 값 채우기
3. 개발 서버 실행
   ```bash
   npm run dev --workspace=@iroum-ax/web
   ```
4. 브라우저에서 <http://localhost:3000> 접속 → 로그인 화면이 표시되어야 함

## 환경 변수

| 키 | 설명 | Phase A 기본 |
|----|------|---------------|
| `BACKEND_BASE_URL` | Go control-plane base URL | `http://localhost:8080` |
| `KEYCLOAK_BASE_URL` | Keycloak 서버 base URL | `http://localhost:8081` |
| `KEYCLOAK_REALM` | Keycloak realm 이름 | `iroum-ax` |
| `KEYCLOAK_CLIENT_ID` | OIDC Public client ID | `iroum-ax-web` |
| `NEXT_PUBLIC_APP_URL` | Next.js 자체 URL (callback 구성용) | `http://localhost:3000` |

## 디렉터리 구조 (Phase A)

```
apps/web/
├── package.json, tsconfig.json, next.config.ts
├── tailwind.config.ts, postcss.config.js, .eslintrc.json
├── README.md, .env.example, .gitignore
└── src/
    ├── app/
    │   ├── layout.tsx, page.tsx, globals.css
    │   ├── login/page.tsx
    │   ├── (dashboard)/
    │   │   ├── layout.tsx
    │   │   ├── page.tsx               # /dashboard → /dashboard/evidence redirect
    │   │   └── evidence/page.tsx      # Phase B placeholder
    │   └── api/auth/
    │       ├── login/route.ts
    │       ├── callback/route.ts
    │       ├── refresh/route.ts
    │       ├── logout/route.ts
    │       └── me/route.ts
    ├── components/
    │   ├── ui/{button,card,toast}.tsx
    │   ├── auth/role-gate.tsx
    │   └── nav/{sidebar,user-menu}.tsx
    ├── lib/
    │   ├── auth.ts                    # 쿠키 상수 + 세션 헬퍼
    │   ├── api-client.ts              # 서버측 백엔드 호출 wrapper
    │   ├── pkce.ts                    # OIDC PKCE 헬퍼
    │   └── utils.ts                   # shadcn/ui cn
    ├── middleware.ts
    └── types/auth.ts
```

## 보안 메모

- 토큰은 HttpOnly·SameSite=Lax 쿠키에 보관되며 클라이언트 JS는 직접 접근 불가
- PKCE state/verifier는 1회용 10분 유효 쿠키로 관리
- 모든 `/api/v1/*` 호출은 서버측 `apiFetch` wrapper를 경유 (Authorization 헤더 자동 첨부)
- 백엔드 RBAC가 최종 인가의 신뢰의 원천 — UI 가시성 제어는 UX 보조이며 보안 경계가 아님
