import Link from "next/link";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

interface LoginPageProps {
  searchParams: Promise<{ error?: string; from?: string }>;
}

// 로그인 메시지 사전 (SPEC-AX-WEB-001 §7.5).
const ERROR_MESSAGES: Record<string, string> = {
  missing_code_or_state: "인증 응답이 올바르지 않습니다. 다시 시도해 주세요.",
  state_mismatch: "보안 검증에 실패했습니다. 다시 로그인해 주세요.",
  missing_verifier: "세션 정보가 만료되었습니다. 다시 시도해 주세요.",
  token_exchange_failed:
    "Keycloak에서 토큰을 가져오지 못했습니다. 관리자에게 문의하세요.",
};

export default async function LoginPage({
  searchParams,
}: LoginPageProps): Promise<React.ReactElement> {
  const params = await searchParams;
  const errorCode = params.error;
  const errorMessage = errorCode
    ? (ERROR_MESSAGES[errorCode] ?? "로그인 중 오류가 발생했습니다.")
    : null;

  return (
    <main className="flex min-h-screen items-center justify-center bg-muted/30 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-2 text-center">
          <CardTitle className="text-2xl">경영평가 시스템</CardTitle>
          <CardDescription>
            한국전력기술 경영평가 PoC 데모 대시보드
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {errorMessage ? (
            <div
              role="alert"
              className="rounded-md border border-destructive bg-destructive/10 px-4 py-3 text-sm text-destructive"
            >
              {errorMessage}
            </div>
          ) : null}
          <p className="text-sm text-muted-foreground">
            Keycloak SSO 계정으로 로그인하세요. 평가자/열람자/관리자 역할에 따라
            접근 가능한 화면이 달라집니다.
          </p>
        </CardContent>
        <CardFooter>
          <Button asChild className="w-full" size="lg">
            <Link href="/api/auth/login">Keycloak으로 로그인</Link>
          </Button>
        </CardFooter>
      </Card>
    </main>
  );
}
