// abac.go — SPEC-AX-AUTH-003 경량 ABAC: 속성 기반 접근 제어 (narrowing-only)
//
// RBAC(AUTH-002)가 허용한 요청에 한해 추가 속성 조건을 평가한다.
// ABAC는 권한을 부여하지 않으며 오직 추가로 거부만 한다(narrowing filter).
// 외부 의존성 0(망분리 호환), AUTH-001/002 코드 무변경.
//
// 평가 규칙:
//  1. authEnabled=false 또는 매칭 정책 없음 → 투과 (REQ-ABAC-009)
//  2. RoleAdmin 보유 → 투과 (REQ-ABAC-004)
//  3. 매칭 정책의 모든 조건 AND 평가; 하나라도 deny → 403 + audit
//  4. ABAC는 절대 allow를 부여하지 않음 (RBAC가 이미 게이트)
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ErrCodeABACDenied — ABAC 거부 응답 에러 코드 (RBAC PERMISSION_DENIED와 구별)
// REQ-ABAC-005: 코드 값으로 RBAC 403과 구별 가능해야 함
const ErrCodeABACDenied = "ABAC_CONDITION_DENIED"

// orgScopePrefix — org_unit을 인코딩하는 scope 토큰 접두사 (D1-A)
// 예: "iroum-ax-org:finance" → org_unit "finance"
// ParseRolesFromScope는 이 토큰을 인식하지 않으므로 RBAC에 영향 없음.
const orgScopePrefix = "iroum-ax-org:"

// ABACCondition — 평가 가능한 단일 속성 조건.
// allow=false이면 reason에 거부 사유(condition:detail)를 담는다.
type ABACCondition interface {
	Evaluate(ctx context.Context, user *User, r *http.Request) (allow bool, reason string)
}

// ABACPolicy — 특정 자원 경로 패턴에 적용되는 조건 묶음.
type ABACPolicy struct {
	// PathPrefix 이 정책이 적용되는 URL 경로 접두사
	PathPrefix string
	// Conditions AND로 평가되는 조건 목록 (하나라도 deny → 정책 deny)
	Conditions []ABACCondition
}

// ABACEvaluator — 정책 집합 평가. Admin 우회 + narrowing-only 보장.
// fieldalignment-friendly: 인터페이스(16B) 먼저, 슬라이스(24B) 마지막
type ABACEvaluator struct {
	recorder auditRecorder
	policies []ABACPolicy
}

// NewABACEvaluator — 정책 집합과 감사 recorder로 평가자를 생성한다.
// recorder가 nil이면 거부 시 감사 기록을 skip한다(AUTH-002 관례 정합).
func NewABACEvaluator(policies []ABACPolicy, recorder auditRecorder) *ABACEvaluator {
	return &ABACEvaluator{policies: policies, recorder: recorder}
}

// Evaluate — RBAC 통과 요청에 대해 ABAC 정책을 평가한다.
//
// 반환: allow=true이면 통과(reason 빈 문자열), allow=false이면 거부(reason=거부사유).
// user가 nil이거나 매칭 정책 없으면 투과(REQ-ABAC-009). Admin이면 투과(REQ-ABAC-004).
//
// @MX:ANCHOR: [AUTO] SPEC-AX-AUTH-003 REQ-ABAC-001~004 모든 ABAC-guarded 요청의 단일 결정 진입점
// @MX:REASON: 미들웨어 / 핸들러-호출 평가자 / 테스트에서 호출 (fan_in >= 3), narrowing-only 불변식의 단일 게이트
func (e *ABACEvaluator) Evaluate(ctx context.Context, user *User, r *http.Request) (allow bool, reason string) {
	// user 없음 → 투과 (RBAC가 이미 게이트, narrowing-only)
	if user == nil {
		return true, ""
	}

	// RoleAdmin 보유 → 모든 ABAC 조건 우회 (REQ-ABAC-004)
	if hasAdminRole(user) {
		return true, ""
	}

	// 매칭 정책 평가 (REQ-ABAC-009: 매칭 없으면 투과)
	for _, p := range e.policies {
		if !strings.HasPrefix(r.URL.Path, p.PathPrefix) {
			continue
		}
		for _, cond := range p.Conditions {
			ok, denyReason := cond.Evaluate(ctx, user, r)
			if !ok {
				return false, denyReason
			}
		}
	}

	return true, ""
}

// hasAdminRole — user의 scope에서 RoleAdmin 보유 여부를 확인한다.
// ParseRolesFromScope(AUTH-002)를 재사용 — scope 단일 진실 원천.
func hasAdminRole(user *User) bool {
	roles := ParseRolesFromScope(strings.Join(user.Scopes, " "))
	for _, role := range roles {
		if role == RoleAdmin {
			return true
		}
	}
	return false
}

// OrgUnitFromUser — user.Scopes에서 "iroum-ax-org:<unit>" 토큰의 unit을 추출한다.
// 없으면 빈 문자열을 반환한다 (D1-A: scope 인코딩).
func OrgUnitFromUser(user *User) string {
	if user == nil {
		return ""
	}
	for _, s := range user.Scopes {
		if unit, ok := strings.CutPrefix(s, orgScopePrefix); ok {
			return unit
		}
	}
	return ""
}

// OwnershipCondition — user.UID vs 요청에서 도출 가능한 자원 소유자 (REQ-ABAC-001).
// 소유자를 도출할 수 없으면 투과한다(AC-001-4: DB 조회는 범위 밖).
type OwnershipCondition struct {
	// OwnerFromRequest 요청에서 소유자 식별자를 도출 (ok=false면 도출 불가 → 투과)
	OwnerFromRequest func(*http.Request) (owner string, ok bool)
}

// Evaluate — UID가 자원 소유자와 일치하면 allow, 불일치면 deny.
func (c OwnershipCondition) Evaluate(_ context.Context, user *User, r *http.Request) (bool, string) {
	if c.OwnerFromRequest == nil {
		return true, "" // 도출자 미설정 → 투과
	}
	owner, ok := c.OwnerFromRequest(r)
	if !ok {
		return true, "" // 소유자 도출 불가 → 투과 (AC-001-4)
	}
	if user.UID != owner {
		return false, "ownership:mismatch"
	}
	return true, ""
}

// OrgUnitCondition — user org_unit vs 자원 org_unit (REQ-ABAC-002).
// 어느 한쪽이라도 없으면 투과한다(AC-002-4).
type OrgUnitCondition struct {
	// ResourceOrg 요청 대상 자원의 조직 단위 (빈 문자열이면 도출 불가 → 투과)
	ResourceOrg func(*http.Request) string
}

// Evaluate — 사용자 org_unit과 자원 org_unit이 일치하면 allow, 불일치면 deny.
func (c OrgUnitCondition) Evaluate(_ context.Context, user *User, r *http.Request) (bool, string) {
	userOrg := OrgUnitFromUser(user)
	if userOrg == "" {
		return true, "" // org_unit 속성 없음 → 투과 (AC-002-4)
	}
	if c.ResourceOrg == nil {
		return true, ""
	}
	resOrg := c.ResourceOrg(r)
	if resOrg == "" {
		return true, "" // 자원 org 도출 불가 → 투과
	}
	if userOrg != resOrg {
		return false, "org_unit:cross-org"
	}
	return true, ""
}

// TimeWindowCondition — KST(고정 UTC+9) 업무 시간 제한 (REQ-ABAC-003).
// [StartHour, EndHour) 범위 밖이면 deny. Admin은 Evaluator에서 이미 우회됨.
// fieldalignment-friendly: 포인터/함수(8B) 먼저, int 마지막
type TimeWindowCondition struct {
	// tz 고정 KST 존 — tzdata 비의존 (D4)
	tz *time.Location
	// now 현재 시각 공급자 (테스트 주입용; 기본 time.Now)
	now func() time.Time
	// StartHour 업무 시작 시각 (포함, KST)
	StartHour int
	// EndHour 업무 종료 시각 (제외, KST)
	EndHour int
}

// NewTimeWindowCondition — KST 업무 시간 조건을 생성한다.
//
// @MX:WARN: [AUTO] time.LoadLocation 사용 금지 — time.FixedZone("KST", 9*3600) 강제
// @MX:REASON: 망분리 scratch 컨테이너에는 tzdata가 없어 LoadLocation("Asia/Seoul")은 런타임 실패 위험 (D4)
func NewTimeWindowCondition(startHour, endHour int) *TimeWindowCondition {
	return &TimeWindowCondition{
		StartHour: startHour,
		EndHour:   endHour,
		tz:        time.FixedZone("KST", 9*3600),
		now:       time.Now,
	}
}

// Evaluate — 현재 KST 시각이 [StartHour, EndHour) 범위 내이면 allow.
func (c *TimeWindowCondition) Evaluate(_ context.Context, _ *User, _ *http.Request) (bool, string) {
	hour := c.now().In(c.tz).Hour()
	if hour < c.StartHour || hour >= c.EndHour {
		return false, "time_window:outside_hours"
	}
	return true, ""
}

// ABACMiddleware — server.go에서 REST handler를 래핑하는 ABAC 미들웨어.
// 실행 순서: authn → authz → abac(여기) → handler (체인 캡슐화로 보장, chain.go 무변경).
//
// @MX:NOTE: [AUTO] SPEC-AX-AUTH-003 체인 삽입점 — server.go가 BuildRESTChain 안쪽에서 REST mux를 래핑
func ABACMiddleware(e *ABACEvaluator, authEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// authEnabled=false → 완전 no-op (REQ-ABAC-009, AC-009-1)
			if !authEnabled {
				next.ServeHTTP(w, r)
				return
			}

			user, _ := UserFromContext(r.Context())
			allow, reason := e.Evaluate(r.Context(), user, r)
			if allow {
				next.ServeHTTP(w, r)
				return
			}

			// ABAC 거부 → 403 + 감사 기록 (REQ-ABAC-005, REQ-ABAC-007)
			writeABACDenied(w, reason)
			if e.recorder != nil && user != nil {
				roles := ParseRolesFromScope(strings.Join(user.Scopes, " "))
				if auditErr := e.recorder.LogForbiddenEvent(
					r.Context(), r.Method, r.URL.Path, reason, user.UID, roles,
				); auditErr != nil {
					// 감사 기록 실패 — 403 응답은 이미 전송됨, 복구 불가; 무시
					_ = auditErr
				}
			}
		})
	}
}

// writeABACDenied — 403 Forbidden 응답 전송 (REQ-ABAC-005).
// AUTH-002 body 형식(error.code/message/details) 유지하되 code는 ABAC 전용.
func writeABACDenied(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	body := forbiddenErrorBody{
		Error: forbiddenErrObj{
			Code:    ErrCodeABACDenied,
			Message: "ABAC condition denied",
			Details: forbiddenDetails{
				Required: reason,
				Granted:  []string{},
			},
		},
	}
	_ = json.NewEncoder(w).Encode(body) //nolint:errcheck // 응답 헤더 전송 후 encode 실패는 복구 불가
}

// DefaultABACPolicies — 기본 ABAC 정책 집합.
// Sprint 0: 빈 슬라이스 반환 — 완전 no-op(투과)로 AUTH 백워드 호환 보존 (REQ-ABAC-009).
// 후속 Sprint에서 자원별 정책을 추가한다.
func DefaultABACPolicies() []ABACPolicy {
	return []ABACPolicy{}
}
