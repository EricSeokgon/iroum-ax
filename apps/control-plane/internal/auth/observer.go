// observer.go — RejectionObserver 인터페이스 정의
// SPEC-AX-OBS-001 Sprint 0 GREEN: 순환 import 해소를 위한 DI 인터페이스
// auth 패키지 내에 선언하여 auth → metrics import를 제거한다.
package auth

// RejectionObserver — JWT 검증 실패 시 reject 이유를 기록하는 관찰자 인터페이스.
//
// auth 패키지가 metrics 패키지를 import하지 않도록 DI(의존성 역전) 패턴으로 선언한다.
// metrics.Metrics 구조체가 이 인터페이스를 구조적으로 만족하며,
// server.go(package main)에서 wire된다.
//
// @MX:ANCHOR: [AUTO] auth reject 관찰자 계약 인터페이스
// @MX:REASON: circular import 해소의 핵심 DI 계약점 — auth가 metrics를 import하지 않도록 역전 (SPEC-AX-OBS-001 §2.0)
type RejectionObserver interface {
	// IncAuthRejection — JWT 검증 실패 시 reason과 함께 호출된다.
	// reason: "expired", "blacklisted", "invalid_signature", "invalid_issuer" 등
	IncAuthRejection(reason string)
}
