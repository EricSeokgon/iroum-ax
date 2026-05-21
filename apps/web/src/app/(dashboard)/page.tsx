import { redirect } from "next/navigation";

// /dashboard 진입 시 기본 화면을 증빙 관리로 안내 (Phase B 이후 구현).
// Phase A 단계에서는 placeholder 역할만 수행한다.

export default function DashboardHomePage(): never {
  redirect("/dashboard/evidence");
}
