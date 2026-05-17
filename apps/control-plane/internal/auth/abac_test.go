// abac_test.go — SPEC-AX-AUTH-003 RED: 경량 ABAC 단위 테스트
// REQ-ABAC-001~009, 테이블 주도 + httptest 통합 검증
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// abacUserReq — 지정된 scope/uid를 가진 User를 context에 주입한 요청 생성
func abacUserReq(method, path, scope, uid string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	u := &User{
		UID:    uid,
		Issuer: "https://issuer.example.com",
		Scopes: splitScope(scope),
	}
	return req.WithContext(WithUser(req.Context(), u))
}

// splitScope — 공백 구분 scope 문자열을 슬라이스로 (테스트 helper)
func splitScope(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	cur := ""
	for _, r := range s {
		if r == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// ---------------------------------------------------------------------------
// REQ-ABAC-001 — OwnershipCondition
// ---------------------------------------------------------------------------

func TestOwnershipCondition_Evaluate(t *testing.T) {
	t.Parallel()

	ownerFromQuery := func(r *http.Request) (string, bool) {
		o := r.URL.Query().Get("owner")
		return o, o != ""
	}

	tests := []struct {
		name       string
		scope      string
		uid        string
		query      string
		wantReason string
		wantAllow  bool
	}{
		{
			name:      "AC-001-1 owner match → allow",
			scope:     "iroum-ax:analyst",
			uid:       "u1",
			query:     "?owner=u1",
			wantAllow: true,
		},
		{
			name:       "AC-001-2 owner mismatch → deny",
			scope:      "iroum-ax:analyst",
			uid:        "u1",
			query:      "?owner=u2",
			wantAllow:  false,
			wantReason: "ownership:mismatch",
		},
		{
			name:      "AC-001-4 owner not derivable → pass-through allow",
			scope:     "iroum-ax:analyst",
			uid:       "u1",
			query:     "",
			wantAllow: true,
		},
	}

	// OwnerFromRequest 미설정 → 투과 (방어 분기)
	t.Run("nil OwnerFromRequest → pass-through allow", func(t *testing.T) {
		t.Parallel()
		cond := OwnershipCondition{}
		req := abacUserReq(http.MethodGet, "/api/v1/workflows/1?owner=u2", "iroum-ax:viewer", "u1")
		user, _ := UserFromContext(req.Context())
		allow, _ := cond.Evaluate(req.Context(), user, req)
		assert.True(t, allow)
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cond := OwnershipCondition{OwnerFromRequest: ownerFromQuery}
			req := abacUserReq(http.MethodGet, "/api/v1/workflows/123"+tc.query, tc.scope, tc.uid)
			user, _ := UserFromContext(req.Context())

			allow, reason := cond.Evaluate(req.Context(), user, req)

			assert.Equal(t, tc.wantAllow, allow)
			if !tc.wantAllow {
				assert.Equal(t, tc.wantReason, reason)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// REQ-ABAC-002 — OrgUnitCondition
// ---------------------------------------------------------------------------

func TestOrgUnitCondition_Evaluate(t *testing.T) {
	t.Parallel()

	resourceOrgFromQuery := func(r *http.Request) string {
		return r.URL.Query().Get("org")
	}

	tests := []struct {
		name       string
		scope      string
		query      string
		wantReason string
		wantAllow  bool
	}{
		{
			name:      "AC-002-1 same org → allow",
			scope:     "iroum-ax:viewer iroum-ax-org:finance",
			query:     "?org=finance",
			wantAllow: true,
		},
		{
			name:       "AC-002-2 cross org → deny",
			scope:      "iroum-ax:viewer iroum-ax-org:finance",
			query:      "?org=hr",
			wantAllow:  false,
			wantReason: "org_unit:cross-org",
		},
		{
			name:      "AC-002-4 org_unit absent → pass-through allow",
			scope:     "iroum-ax:viewer",
			query:     "?org=hr",
			wantAllow: true,
		},
		{
			name:      "resource org absent → pass-through allow",
			scope:     "iroum-ax:viewer iroum-ax-org:finance",
			query:     "",
			wantAllow: true,
		},
	}

	// ResourceOrg 미설정 → 투과 (방어 분기)
	t.Run("nil ResourceOrg → pass-through allow", func(t *testing.T) {
		t.Parallel()
		cond := OrgUnitCondition{}
		req := abacUserReq(http.MethodGet, "/api/v1/workflows", "iroum-ax:viewer iroum-ax-org:finance", "u1")
		user, _ := UserFromContext(req.Context())
		allow, _ := cond.Evaluate(req.Context(), user, req)
		assert.True(t, allow)
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cond := OrgUnitCondition{ResourceOrg: resourceOrgFromQuery}
			req := abacUserReq(http.MethodGet, "/api/v1/workflows"+tc.query, tc.scope, "u1")
			user, _ := UserFromContext(req.Context())

			allow, reason := cond.Evaluate(req.Context(), user, req)

			assert.Equal(t, tc.wantAllow, allow)
			if !tc.wantAllow {
				assert.Equal(t, tc.wantReason, reason)
			}
		})
	}
}

func TestOrgUnitFromScope(t *testing.T) {
	t.Parallel()
	// nil user → 빈 문자열 (방어 분기)
	assert.Equal(t, "", OrgUnitFromUser(nil))

	tests := []struct {
		name  string
		want  string
		scope []string
	}{
		{"present", "finance", []string{"iroum-ax:viewer", "iroum-ax-org:finance"}},
		{"absent", "", []string{"iroum-ax:viewer"}},
		{"empty", "", nil},
		{"only org", "hr", []string{"iroum-ax-org:hr"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := &User{Scopes: tc.scope}
			assert.Equal(t, tc.want, OrgUnitFromUser(u))
		})
	}
}

// ---------------------------------------------------------------------------
// REQ-ABAC-003 — TimeWindowCondition (KST fixed zone, no tzdata)
// ---------------------------------------------------------------------------

func TestTimeWindowCondition_Evaluate(t *testing.T) {
	t.Parallel()

	kst := time.FixedZone("KST", 9*3600)

	tests := []struct {
		at        time.Time
		name      string
		wantAllow bool
	}{
		{
			name:      "AC-003-1 10:00 KST → allow",
			at:        time.Date(2026, 5, 18, 10, 0, 0, 0, kst),
			wantAllow: true,
		},
		{
			name:      "AC-003-2 08:59 KST → deny",
			at:        time.Date(2026, 5, 18, 8, 59, 0, 0, kst),
			wantAllow: false,
		},
		{
			name:      "AC-003-3 18:01 KST → deny",
			at:        time.Date(2026, 5, 18, 18, 1, 0, 0, kst),
			wantAllow: false,
		},
		{
			name:      "boundary 09:00 KST → allow",
			at:        time.Date(2026, 5, 18, 9, 0, 0, 0, kst),
			wantAllow: true,
		},
		{
			name:      "boundary 17:59 KST → allow",
			at:        time.Date(2026, 5, 18, 17, 59, 0, 0, kst),
			wantAllow: true,
		},
		{
			name:      "boundary 18:00 KST → deny (exclusive end)",
			at:        time.Date(2026, 5, 18, 18, 0, 0, 0, kst),
			wantAllow: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cond := NewTimeWindowCondition(9, 18)
			cond.now = func() time.Time { return tc.at }

			req := abacUserReq(http.MethodGet, "/api/v1/workflows", "iroum-ax:viewer", "u1")
			user, _ := UserFromContext(req.Context())

			allow, reason := cond.Evaluate(req.Context(), user, req)

			assert.Equal(t, tc.wantAllow, allow)
			if !tc.wantAllow {
				assert.Equal(t, "time_window:outside_hours", reason)
			}
		})
	}
}

// AC-003-5 — KST 결정이 tzdata 없이도 런타임 에러 없이 동작
func TestTimeWindowCondition_KSTFixedZone(t *testing.T) {
	t.Parallel()
	cond := NewTimeWindowCondition(9, 18)
	// FixedZone은 tzdata 비의존 — UTC 02:30 == KST 11:30 (업무시간 내)
	cond.now = func() time.Time { return time.Date(2026, 5, 18, 2, 30, 0, 0, time.UTC) }

	req := abacUserReq(http.MethodGet, "/api/v1/workflows", "iroum-ax:viewer", "u1")
	user, _ := UserFromContext(req.Context())

	allow, _ := cond.Evaluate(req.Context(), user, req)
	assert.True(t, allow, "UTC 02:30 == KST 11:30 should be within business hours")
}

// ---------------------------------------------------------------------------
// REQ-ABAC-004 — Admin bypass
// ---------------------------------------------------------------------------

func TestABACEvaluator_AdminBypass(t *testing.T) {
	t.Parallel()

	// 항상 거부하는 조건으로 정책 구성
	denyAll := conditionFunc(func(_ context.Context, _ *User, _ *http.Request) (bool, string) {
		return false, "always:deny"
	})
	eval := NewABACEvaluator([]ABACPolicy{
		{PathPrefix: "/api/v1/", Conditions: []ABACCondition{denyAll}},
	}, nil)

	t.Run("AC-004-1 admin violates all conditions → allow", func(t *testing.T) {
		t.Parallel()
		req := abacUserReq(http.MethodGet, "/api/v1/workflows", "iroum-ax:admin", "admin1")
		user, _ := UserFromContext(req.Context())
		allow, _ := eval.Evaluate(req.Context(), user, req)
		assert.True(t, allow)
	})

	t.Run("AC-004-2 non-admin violates condition → deny", func(t *testing.T) {
		t.Parallel()
		req := abacUserReq(http.MethodGet, "/api/v1/workflows", "iroum-ax:viewer", "v1")
		user, _ := UserFromContext(req.Context())
		allow, reason := eval.Evaluate(req.Context(), user, req)
		assert.False(t, allow)
		assert.Equal(t, "always:deny", reason)
	})
}

// ---------------------------------------------------------------------------
// REQ-ABAC-009 — fail-safe / no-op
// ---------------------------------------------------------------------------

func TestABACEvaluator_NoPolicy(t *testing.T) {
	t.Parallel()
	// AC-009-2: 매칭 정책 없음 → 투과 allow
	eval := NewABACEvaluator(nil, nil)
	req := abacUserReq(http.MethodGet, "/api/v1/workflows", "iroum-ax:viewer", "v1")
	user, _ := UserFromContext(req.Context())
	allow, reason := eval.Evaluate(req.Context(), user, req)
	assert.True(t, allow)
	assert.Empty(t, reason)
}

func TestABACEvaluator_NoMatchingPolicy(t *testing.T) {
	t.Parallel()
	denyAll := conditionFunc(func(_ context.Context, _ *User, _ *http.Request) (bool, string) {
		return false, "always:deny"
	})
	// 정책은 /admin/ 에만 적용 — /api/v1/ 요청은 매칭 안 됨 → 투과
	eval := NewABACEvaluator([]ABACPolicy{
		{PathPrefix: "/admin/", Conditions: []ABACCondition{denyAll}},
	}, nil)
	req := abacUserReq(http.MethodGet, "/api/v1/workflows", "iroum-ax:viewer", "v1")
	user, _ := UserFromContext(req.Context())
	allow, _ := eval.Evaluate(req.Context(), user, req)
	assert.True(t, allow)
}

func TestABACEvaluator_NilUser(t *testing.T) {
	t.Parallel()
	// user가 nil이면 ABAC는 평가하지 않고 투과 (RBAC가 이미 게이트, narrowing-only)
	denyAll := conditionFunc(func(_ context.Context, _ *User, _ *http.Request) (bool, string) {
		return false, "always:deny"
	})
	eval := NewABACEvaluator([]ABACPolicy{
		{PathPrefix: "/api/v1/", Conditions: []ABACCondition{denyAll}},
	}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	allow, _ := eval.Evaluate(req.Context(), nil, req)
	assert.True(t, allow)
}

// ---------------------------------------------------------------------------
// REQ-ABAC-005 / REQ-ABAC-007 — Middleware integration + audit
// ---------------------------------------------------------------------------

func TestABACMiddleware_Integration(t *testing.T) {
	t.Parallel()

	ownerFromQuery := func(r *http.Request) (string, bool) {
		o := r.URL.Query().Get("owner")
		return o, o != ""
	}
	policies := []ABACPolicy{
		{
			PathPrefix: "/api/v1/workflows",
			Conditions: []ABACCondition{OwnershipCondition{OwnerFromRequest: ownerFromQuery}},
		},
	}

	t.Run("authEnabled=false → no-op pass-through (AC-009-1)", func(t *testing.T) {
		t.Parallel()
		rec := newCaptureRecorder()
		eval := NewABACEvaluator(policies, rec)
		called := false
		h := ABACMiddleware(eval, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		// owner mismatch이지만 authEnabled=false → 투과
		req := abacUserReq(http.MethodGet, "/api/v1/workflows?owner=u2", "iroum-ax:viewer", "u1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.True(t, called)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, rec.tx.events)
	})

	t.Run("owner match → handler called (AC-001-1)", func(t *testing.T) {
		t.Parallel()
		eval := NewABACEvaluator(policies, nil)
		called := false
		h := ABACMiddleware(eval, true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		req := abacUserReq(http.MethodGet, "/api/v1/workflows?owner=u1", "iroum-ax:analyst", "u1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.True(t, called)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("owner mismatch → 403 ABAC_CONDITION_DENIED + audit (AC-001-2, AC-007-1)", func(t *testing.T) {
		t.Parallel()
		rec := newCaptureRecorder()
		eval := NewABACEvaluator(policies, rec)
		called := false
		h := ABACMiddleware(eval, true)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called = true
		}))
		req := abacUserReq(http.MethodGet, "/api/v1/workflows?owner=u2", "iroum-ax:analyst", "u1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		assert.False(t, called, "handler must not be called when ABAC denies")
		assert.Equal(t, http.StatusForbidden, w.Code)

		var body forbiddenErrorBody
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, ErrCodeABACDenied, body.Error.Code)
		assert.Equal(t, ErrCodeABACDenied, "ABAC_CONDITION_DENIED")

		// audit row 기록 확인 (AC-007-1)
		require.Len(t, rec.tx.events, 1)
		assert.Equal(t, "u1", rec.tx.events[0].UserID)
	})

	t.Run("admin bypass via middleware (AC-001-3)", func(t *testing.T) {
		t.Parallel()
		eval := NewABACEvaluator(policies, nil)
		called := false
		h := ABACMiddleware(eval, true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		req := abacUserReq(http.MethodGet, "/api/v1/workflows?owner=u2", "iroum-ax:admin", "admin1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.True(t, called)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("nil recorder → 403 without panic (AC-007-2)", func(t *testing.T) {
		t.Parallel()
		eval := NewABACEvaluator(policies, nil)
		h := ABACMiddleware(eval, true)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		req := abacUserReq(http.MethodGet, "/api/v1/workflows?owner=u2", "iroum-ax:viewer", "u1")
		w := httptest.NewRecorder()
		assert.NotPanics(t, func() { h.ServeHTTP(w, req) })
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// AC-005-3 — 거부 응답이 AUTH-002 body 형식(error.code/message/details) 유지
func TestABACConditionDeniedResponse(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeABACDenied(w, "ownership:mismatch")

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body forbiddenErrorBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ABAC_CONDITION_DENIED", body.Error.Code)
	assert.NotEmpty(t, body.Error.Message)
	assert.Equal(t, "ownership:mismatch", body.Error.Details.Required)
}

// REQ-ABAC-007 D5 — ActionABACDenied 상수 존재 확인
func TestActionABACDeniedConstant(t *testing.T) {
	t.Parallel()
	assert.Equal(t, audit.Action("ABAC_CONDITION_DENIED"), audit.ActionABACDenied)
}

// DefaultABACPolicies는 빈 슬라이스(또는 정책 집합)를 반환 — no-op 보장
func TestDefaultABACPolicies(t *testing.T) {
	t.Parallel()
	pols := DefaultABACPolicies()
	// 빈 슬라이스 허용 (Sprint 0). nil 또는 길이 0이면 완전 no-op.
	eval := NewABACEvaluator(pols, nil)
	req := abacUserReq(http.MethodGet, "/api/v1/workflows", "iroum-ax:viewer", "u1")
	user, _ := UserFromContext(req.Context())
	allow, _ := eval.Evaluate(req.Context(), user, req)
	assert.True(t, allow, "default policies must not deny RBAC-passed requests by default")
}

// conditionFunc — 테스트용 인라인 ABACCondition 어댑터
type conditionFunc func(context.Context, *User, *http.Request) (bool, string)

func (f conditionFunc) Evaluate(ctx context.Context, u *User, r *http.Request) (bool, string) {
	return f(ctx, u, r)
}
