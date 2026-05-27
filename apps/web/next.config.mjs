// @MX:NOTE: Next.js 설정 — Go control-plane(:8080)을 위한 rewrite proxy 정의.
// SPEC-AX-WEB-001 §7.2: BFF Route Handler(`/api/auth/**`)는 직접 처리,
// 그 외 `/api/v1/**`는 Go control-plane으로 투과 프록시.

const BACKEND_BASE_URL =
  process.env["BACKEND_BASE_URL"] ?? "http://localhost:8080";

const nextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,

  // 한국어 단일 (i18n 라이브러리 미도입, §3 비목표 #3)
  // 정적 ko 메시지는 lib/i18n/ko.ts에서 관리

  async rewrites() {
    return [
      {
        // SPEC §1.3 카탈로그 31 endpoints의 base path
        // BFF 라우트(`/api/auth/**`)는 우선순위로 Next.js Route Handler가 처리하므로 영향 없음
        source: "/api/v1/:path*",
        destination: `${BACKEND_BASE_URL}/api/v1/:path*`,
      },
    ];
  },

  async headers() {
    return [
      {
        // 보수적 CSP — SPEC §7.8 보안 가드
        source: "/(.*)",
        headers: [
          { key: "X-Frame-Options", value: "DENY" },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
        ],
      },
    ];
  },
};

export default nextConfig;
