// SPEC-AX-E2E-001 — 한국어 텍스트 셀렉터 상수화 (OPEN #6 옵션 A)
// SUT data-testid 추가 없이 (REQ-E2E-NO-300, EXC-6) 텍스트/role 기반으로 안정성 확보.
// SUT UI 텍스트 변경 시 본 파일 한 곳만 수정.

export const LABELS = {
  // 인증 관련
  logout: "로그아웃",
  loginButton: "로그인",

  // 증빙
  upload: "업로드",

  // 점수
  scoreSubmit: "저장",

  // 리뷰
  reviewSubmit: "+ 리뷰 제출",
  approve: "승인",
  reject: "반려",
  assign: "리뷰어 배정",

  // 루브릭
  edit: "수정",
  save: "저장",

  // 권한 차단 (in-page 메시지)
  accessDenied: "권한이 없습니다. 관리자만 접근할 수 있습니다.",
} as const;

/**
 * Playwright 셀렉터 빌더 — 자주 쓰는 패턴을 함수로 묶어둠.
 * 사용 예: `page.locator(SEL.button(LABELS.upload))`
 */
export const SEL = {
  /** has-text 기반 button 셀렉터 */
  button: (label: string): string => `button:has-text("${label}")`,

  /** 일반 텍스트 셀렉터 */
  text: (label: string): string => `text=${label}`,

  /** type=submit 버튼 */
  submitButton: (): string => 'button[type="submit"]',

  /** number 인풋 */
  numberInput: (): string => 'input[type="number"]',
} as const;
