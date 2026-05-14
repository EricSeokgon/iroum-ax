// Package auth — Go/Python 공유 JWT 클레임 구조체
// SPEC-AX-AUTH-001 §2.3 Shared pkg/auth/
package auth

import "time"

// Claims — JWT 표준 클레임 + iroum-ax 확장 클레임 공유 구조체
//
// Go control-plane과 Python pipelines 양쪽에서 동일한 클레임 이름을 사용한다.
// Python 등가물: pipelines/auth/models.py Claims (Pydantic 모델)
//
// @MX:NOTE: [AUTO] Go-Python 공유 클레임 구조체 — 변경 시 pipelines/auth/models.py와 반드시 동기화
// @MX:SPEC: SPEC-AX-AUTH-001 §2.3
//
// fieldalignment: slice 먼저, string, time.Time 마지막
type Claims struct {
	ExpiresAt time.Time `json:"exp"`
	IssuedAt  time.Time `json:"iat"`
	NotBefore time.Time `json:"nbf,omitempty"`
	Subject   string    `json:"sub"`
	Issuer    string    `json:"iss"`
	JWTID     string    `json:"jti,omitempty"`
	TokenType string    `json:"typ,omitempty"`
	Scope     string    `json:"scope,omitempty"`
	Audience  []string  `json:"aud,omitempty"`
	Roles     []string  `json:"roles,omitempty"`
}
