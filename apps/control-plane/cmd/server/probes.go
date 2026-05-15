// probes.go — liveness(/health) + readiness(/ready) HTTP 핸들러 + gRPC Health 구현
// SPEC-AX-SERVER-001 S1 deliverable (REQ-SERVER-004)
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// buildVersion 빌드 시점에 ldflags로 주입; 기본값은 "dev"
var buildVersion = "dev"

// livenessHandler GET /health — liveness probe.
// 의존성 상태와 무관하게 항상 HTTP 200을 반환한다 (K8s livenessProbe 용도).
// REQ-SERVER-004-E1
func livenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]string{
		"status":  "ok",
		"service": "iroum-ax-control-plane",
		"version": buildVersion,
	}
	_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// readyCheck 단일 readiness 체크 결과
type readyCheck struct {
	err  error
	name string
}

// readinessHandler GET /ready — readiness probe 핸들러 팩토리.
//
// @MX:ANCHOR: [AUTO] liveness/readiness 검사 로직의 단일 출처
// @MX:REASON: REST GET /ready + gRPC Health.Check, 헬퍼를 공유하여 일관성 유지
func readinessHandler(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// shutdown 중이면 즉시 503 반환 (REQ-SERVER-004-S1)
		if s.isShuttingDown() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "shutting_down"}) //nolint:errcheck
			return
		}

		timeout := time.Duration(s.cfg.ReadyProbeTimeoutSeconds) * time.Second
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		checks := runReadyChecks(ctx, s)

		allOK := true
		checksResult := make(map[string]string, len(checks))
		for _, c := range checks {
			if c.err != nil {
				allOK = false
				if isTimeoutErr(c.err) {
					checksResult[c.name] = "failed: timeout exceeded " +
						time.Duration(s.cfg.ReadyProbeTimeoutSeconds).String() + "s"
				} else {
					checksResult[c.name] = "failed: " + c.err.Error()
				}
			} else {
				checksResult[c.name] = "ok"
			}
		}

		if allOK {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"status": "ready",
				"checks": checksResult,
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"status": "not_ready",
				"checks": checksResult,
			})
		}
	}
}

// runReadyChecks 병렬로 readiness 체크를 수행한다.
func runReadyChecks(ctx context.Context, s *Server) []readyCheck {
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		checks []readyCheck
	)

	addCheck := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := fn()
			mu.Lock()
			checks = append(checks, readyCheck{name: name, err: err})
			mu.Unlock()
		}()
	}

	// (i) PostgreSQL ping
	addCheck("postgres", func() error {
		if s.pgStore == nil {
			return nil
		}
		return s.pgStore.Ping(ctx)
	})

	// (ii) Redis ping
	addCheck("redis", func() error {
		if s.redisClient == nil {
			return nil
		}
		return s.redisClient.Ping(ctx).Err()
	})

	// (iii) JWKS reachable (AuthEnabled=true 인 경우만)
	if s.cfg.AuthEnabled && s.jwksCache != nil {
		addCheck("oidc", func() error {
			if !s.jwksCache.Reachable(ctx) {
				return errJWKSUnreachable
			}
			return nil
		})
	}

	wg.Wait()
	return checks
}

// errJWKSUnreachable JWKS 캐시가 unreachable 상태임을 나타내는 sentinel
var errJWKSUnreachable = &jwksUnreachableError{}

type jwksUnreachableError struct{}

func (e *jwksUnreachableError) Error() string { return "jwks unreachable" }

// isTimeoutErr context deadline exceeded 여부 판정
func isTimeoutErr(err error) bool {
	return err != nil && (err == context.DeadlineExceeded || err.Error() == "context deadline exceeded")
}

// shutting down 여부를 Server가 추적하는 플래그
// shutdown()이 호출되면 true로 설정 (probes.go와 공유)
var _ = (*Server)(nil) // 컴파일 타임 참조 유지

// isShuttingDown shutting down 상태 여부를 반환한다.
// 현재는 shutdownOnce의 실행 여부를 추적하는 별도 플래그로 구현.
// S2에서 atomic.Bool 기반으로 정교화 예정.
func (s *Server) isShuttingDown() bool {
	// shutdownOnce 사용 여부를 확인할 수 없으므로 별도 plg 필드로 추적
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shuttingDown
}

// ────────────────────────────────────────────────────────────
// gRPC Health Server 구현
// ────────────────────────────────────────────────────────────

// grpcHealthServer grpc_health_v1.HealthServer 구현체.
// runReadyChecks와 동일한 체크를 수행하여 일관성을 유지한다.
type grpcHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	srv *Server
}

// Check gRPC health check (REQ-SERVER-004-E3).
func (h *grpcHealthServer) Check(ctx context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if h.srv.isShuttingDown() {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	timeout := time.Duration(h.srv.cfg.ReadyProbeTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	checks := runReadyChecks(ctx, h.srv)
	for _, c := range checks {
		if c.err != nil {
			h.srv.logger.Warn("gRPC health check 실패",
				zap.String("check", c.name),
				zap.Error(c.err),
			)
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}
	}
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// Watch gRPC health watch — 미구현 (REQ-SERVER-004-E3에서 Watch는 unimplemented 반환)
func (h *grpcHealthServer) Watch(_ *grpc_health_v1.HealthCheckRequest, _ grpc_health_v1.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}
