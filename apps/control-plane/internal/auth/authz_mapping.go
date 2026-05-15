// authz_mapping.go — method/path → Permission 매핑 테이블
// Sprint 0 GREEN: SPEC-AX-AUTH-002 REQ-AUTH2-001-E1/E2/U1
package auth

import "strings"

// restEntry — REST 매핑 테이블 엔트리 (정적 배열로 선형 탐색)
// fieldalignment-friendly: bool은 맨 마지막
type restEntry struct {
	// method HTTP 메서드 (대문자)
	method string
	// pathPrefix 경로 접두사 — 정확 매칭 또는 prefix 매칭
	pathPrefix string
	// perm 요구 권한 문자열
	perm string
	// isPrefix true이면 prefix 매칭, false이면 정확 매칭
	isPrefix bool
	// bypass true이면 인증/인가 없이 통과
	bypass bool
}

// restPermissionTable — 정규화된 REST 매핑 테이블 (REQ-AUTH2-001-E1)
// 코드가 canonical source-of-truth (REQ-AUTH2-001-S1: runtime config 변경 금지)
//
// @MX:ANCHOR: [AUTO] REST/gRPC 권한 매핑 단일 진입점 (fan_in >= 4)
// @MX:REASON: REQ-AUTH2-001-U1 (Unwanted default-deny) + REQ-AUTH2-001-S1 (Ubiquitous code-as-config) 보안 결정의 immutable source-of-truth
var restPermissionTable = []restEntry{
	// bypass 경로: 인증/인가 없이 통과
	{method: "GET", pathPrefix: "/health", perm: "", bypass: true},
	{method: "HEAD", pathPrefix: "", perm: "", bypass: true, isPrefix: true},    // HEAD * bypass
	{method: "OPTIONS", pathPrefix: "", perm: "", bypass: true, isPrefix: true}, // OPTIONS * bypass (CORS preflight)

	// POST /api/v1/workflows → write:workflow (admin, analyst)
	{method: "POST", pathPrefix: "/api/v1/workflows", perm: "write:workflow"},

	// GET /api/v1/workflows → read:workflow (admin, analyst, viewer)
	{method: "GET", pathPrefix: "/api/v1/workflows", perm: "read:workflow"},

	// DELETE /api/v1/workflows/{id} → delete:workflow (admin only)
	// prefix 매칭: "/api/v1/workflows/" 로 시작하는 DELETE
	{method: "DELETE", pathPrefix: "/api/v1/workflows/", perm: "delete:workflow", isPrefix: true},

	// GET /api/v1/workflows/{id} → read:workflow
	// prefix 매칭: "/api/v1/workflows/" 로 시작하는 GET (목록 경로보다 먼저 평가하지 않도록 주의)
	// 위 GET /api/v1/workflows (정확 매칭) 이후에 배치
	{method: "GET", pathPrefix: "/api/v1/workflows/", perm: "read:workflow", isPrefix: true},

	// POST /api/v1/recommendations/{id}/feedback → write:recommendation
	{method: "POST", pathPrefix: "/api/v1/recommendations/", perm: "write:recommendation", isPrefix: true},

	// POST /api/v1/documents/upload → write:workflow (업로드는 워크플로우 생성 selfsame)
	{method: "POST", pathPrefix: "/api/v1/documents/upload", perm: "write:workflow"},
}

// grpcPermissionTable — gRPC FullMethod → Permission 매핑 (REQ-AUTH2-001-E2)
// 코드가 canonical source-of-truth (REQ-AUTH2-001-S1)
var grpcPermissionTable = map[string]string{
	"/iroum.ax.v1.WorkflowService/CreateWorkflow": "write:workflow",
	"/iroum.ax.v1.WorkflowService/GetWorkflow":    "read:workflow",
	"/iroum.ax.v1.WorkflowService/ListWorkflows":  "read:workflow",
}

// grpcBypassMethods — gRPC bypass 메서드 (인증/인가 없이 통과)
var grpcBypassMethods = map[string]bool{
	"/grpc.health.v1.Health/Check": true,
}

// LookupRESTPermission — method + path를 받아 required Permission을 반환한다.
//
// 반환값:
//   - perm: 요구 권한 문자열 (bypass이면 빈 문자열)
//   - bypass: true이면 인증/인가 없이 통과
//   - found: true이면 매핑 발견 (false이면 default-deny → 호출자는 503 반환)
//
// HEAD/OPTIONS: 모든 경로 bypass
// /health: GET만 bypass
// 그 외 미매핑 경로: found=false (REQ-AUTH2-001-U1 default-deny)
//
// @MX:ANCHOR: [AUTO] REST 권한 매핑 단일 진입점 (fan_in >= 4: RESTAuthzMiddleware/테스트/체인/audit)
// @MX:REASON: REQ-AUTH2-001-U1 default-deny safety net의 핵심 결정 지점
func LookupRESTPermission(method, path string) (perm string, bypass bool, found bool) {
	// HEAD / OPTIONS는 전체 bypass (CORS preflight 포함)
	if method == "HEAD" || method == "OPTIONS" {
		return "", true, true
	}

	for _, e := range restPermissionTable {
		// HEAD/OPTIONS 전체 bypass 항목은 위에서 처리됨, 여기선 스킵
		if e.isPrefix && e.pathPrefix == "" {
			continue
		}

		if e.method != method {
			continue
		}

		var matched bool
		if e.isPrefix {
			matched = strings.HasPrefix(path, e.pathPrefix)
		} else {
			matched = path == e.pathPrefix
		}

		if !matched {
			continue
		}

		return e.perm, e.bypass, true
	}

	// 매핑 없음 → default-deny (REQ-AUTH2-001-U1)
	return "", false, false
}

// LookupGRPCPermission — gRPC FullMethod를 받아 required Permission을 반환한다.
//
// 반환값:
//   - perm: 요구 권한 문자열 (bypass이면 빈 문자열)
//   - bypass: true이면 인증/인가 없이 통과
//   - found: true이면 매핑 발견
func LookupGRPCPermission(fullMethod string) (perm string, bypass bool, found bool) {
	// bypass 메서드 확인 (Health Check 등)
	if grpcBypassMethods[fullMethod] {
		return "", true, true
	}

	perm, ok := grpcPermissionTable[fullMethod]
	if !ok {
		// 매핑 없음 → default-deny
		return "", false, false
	}
	return perm, false, true
}
