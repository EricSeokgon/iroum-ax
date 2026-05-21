import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * shadcn/ui 표준 className 결합 헬퍼.
 * Tailwind 충돌 클래스는 twMerge로 후순위 우선 적용.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
