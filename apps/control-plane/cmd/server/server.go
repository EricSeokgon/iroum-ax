// server.go — 서버 부트스트랩 & Dual Listener (gRPC :50051 + REST :8080)
// SPEC-AX-SERVER-001 S0+S1 Sprint: New() + Run() + shutdown() 구현
// package main: 이전 package server에서 전환 (Evaluator INFO D1 대응)
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/auth"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/config"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/metrics"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/observability"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/proto"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/scheduler"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/server"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/workflow"
)

// Server gRPC + REST 복합 서버 구조체.
// 필드 순서: 포인터/인터페이스(8B) 우선 배치 (fieldalignment 최적화).
//
// @MX:ANCHOR: [AUTO] 서버 부트스트랩 + dual listener의 단일 진입점
// @MX:REASON: main.go + server_test.go 2곳에서 참조 — dependency wiring의 단일 출처
type Server struct {
	startedAt      time.Time
	sm             *workflow.StateMachine
	workflowSvc    *server.WorkflowService
	redisClient    *redis.Client
	oidcClient     *auth.OIDCClient
	jwksCache      *auth.JWKSCache
	tokenValidator *auth.TokenValidator
	refreshStore   *auth.RedisRefreshStore
	recorder       *audit.Recorder
	txCoord        *workflow.TxCoordinator
	cfg            *config.Config
	pgStore        *store.PgWorkflowStore
	restHandler    *server.RESTHandler
	dispatcher     *scheduler.CeleryDispatcher
	grpcServer     *grpc.Server
	httpServer     *http.Server
	logger         *zap.Logger
	// tracerShutdown — OTel TracerProvider graceful shutdown 클로저 (Sprint 2)
	// @MX:NOTE: [AUTO] InitTracer가 반환한 shutdown 클로저 — server.shutdown() defer 체인에 등록
	tracerShutdown func(context.Context) error
	restAddr       string
	grpcAddr       string
	mu             sync.RWMutex
	shutdownOnce   sync.Once
	shuttingDown   bool
}

// New 의존성을 주입하며 Server를 초기화한다.
// REQ-SERVER-UBI-001-b 단계 (c)~(j) 순서를 강제한다.
// 단계 (a) config.Load() / (b) logger.New()는 main.go에서 선행 호출.
//
// @MX:ANCHOR: [AUTO] 서버 의존성 주입 진입점
// @MX:REASON: main.go + server_test.go 2곳에서 호출
func New(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*Server, error) {
	s := &Server{cfg: cfg, logger: logger}

	// 단계 (b-1): OTel TracerProvider 초기화 (망분리 정합: noop default)
	// OTEL_EXPORTER_OTLP_ENDPOINT 미설정 시 NeverSample → 외부 전송 0 (REQ-OBS-UBI-001-a)
	// @MX:NOTE: [AUTO] InitTracer는 otel.SetTracerProvider(전역 등록) + shutdown 클로저 반환
	_, tracerShutdown, err := observability.InitTracer(ctx)
	if err != nil {
		return nil, fmt.Errorf("init step otel_tracer failed: %w", err)
	}
	s.tracerShutdown = tracerShutdown

	// 단계 (c): PgWorkflowStore 초기화 + Ping 검증
	pgStore, err := store.NewPgWorkflowStore(ctx, cfg.PostgresDSN, logger)
	if err != nil {
		_ = tracerShutdown(ctx) //nolint:errcheck
		return nil, fmt.Errorf("init step pg_store failed: %w", err)
	}
	s.pgStore = pgStore

	// 단계 (c) — pg_ping: 명시적 readiness 재확인
	if err := pgStore.Ping(ctx); err != nil {
		pgStore.Close()
		_ = tracerShutdown(ctx) //nolint:errcheck
		return nil, fmt.Errorf("init step pg_ping failed: %w", err)
	}

	// 단계 (c-1): pg pool GaugeFunc 등록 (pgStore 초기화 이후 — REQ-OBS-001)
	// @MX:NOTE: [AUTO] pg pool 연결 수 GaugeFunc — pgStore 의존으로 순서 고정
	metrics.RegisterPgPoolGauge(metrics.Registry(), func() float64 {
		stat := pgStore.PoolStats()
		if stat == nil {
			return 0
		}
		return float64(stat.AcquiredConns())
	})

	// 단계 (d): Redis 클라이언트 초기화 + Ping 검증
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close() //nolint:errcheck // cleanup 에러는 root cause를 가리지 않도록 무시
		pgStore.Close()
		return nil, fmt.Errorf("init step redis_ping failed: %w", err)
	}
	s.redisClient = redisClient

	// 단계 (e): AuthEnabled=true인 경우만 OIDC + JWKS 초기화
	if cfg.AuthEnabled {
		oidcClient, err := auth.NewOIDCClient(ctx, cfg.OIDCIssuerURL)
		if err != nil {
			_ = redisClient.Close() //nolint:errcheck
			pgStore.Close()
			return nil, fmt.Errorf("init step oidc_client failed: %w", err)
		}
		s.oidcClient = oidcClient

		// JWKS 캐시 생성 및 첫 fetch 강제 (warm-up)
		jwksURI := oidcClient.GetMetadata().JWKSUri
		jwksTTL := time.Duration(cfg.JWKSCacheTTLSeconds) * time.Second
		jwksStale := time.Duration(cfg.JWKSStaleMaxAgeSeconds) * time.Second
		jc := auth.NewJWKSCache(jwksURI,
			auth.WithCacheTTL(jwksTTL),
			auth.WithStaleMaxAge(jwksStale),
		)
		// warm-up: 첫 fetch 강제 — jwksCache.Reachable이 true가 되도록
		if _, _, _, wErr := jc.GetKey(ctx, "warmup-probe"); wErr != nil {
			// GetKey는 kid 미존재 에러도 반환하지만, fetch 자체는 성공할 수 있음.
			// jwks fetch 실패(network error 등)인지 구분한다.
			if !errors.Is(wErr, auth.ErrJWKSUnavailable) {
				// kid 미존재는 정상 (키가 로드됨), fetch 성공으로 간주
				_ = wErr //nolint:errcheck
			} else {
				_ = redisClient.Close() //nolint:errcheck
				pgStore.Close()
				return nil, fmt.Errorf("init step jwks_warmup failed: %w", wErr)
			}
		}
		s.jwksCache = jc

		// 단계 (f): TokenValidator + RefreshStore
		// @MX:NOTE: [AUTO] WithRejectionObserver(metrics.GlobalMetrics()) — auth→metrics 순환 import 해소를 위한 DI 진입점
		// @MX:ANCHOR: [AUTO] TokenValidator 의존성 주입 지점 (server.go) — auth.RejectionObserver DI 계약
		// @MX:REASON: auth 패키지가 metrics import 금지 → server.go(package main)에서만 둘을 연결 가능
		tv, err := auth.New(ctx, cfg.OIDCIssuerURL, cfg.OIDCAudience,
			auth.WithJWKSProvider(jc),
			auth.WithRejectionObserver(metrics.GlobalMetrics()),
		)
		if err != nil {
			_ = redisClient.Close() //nolint:errcheck
			pgStore.Close()
			return nil, fmt.Errorf("init step token_validator failed: %w", err)
		}
		s.tokenValidator = tv
	}

	// 단계 (f) — refresh_store: AuthEnabled 무관하게 생성
	s.refreshStore = auth.NewRedisRefreshStore(cfg.RedisAddr)

	// 단계 (g): Recorder → TxCoordinator → StateMachine
	rec := audit.NewRecorder(cfg.AuthEnabled)
	s.recorder = rec

	txCoord := workflow.NewTxCoordinator(pgStore, rec)
	s.txCoord = txCoord

	sm := workflow.NewStateMachine(txCoord, logger)
	s.sm = sm

	// 단계 (h): RedisClientAdapter → CeleryDispatcher
	redisAdapter := scheduler.NewRedisClientAdapter(redisClient)
	hostname, _ := os.Hostname() //nolint:errcheck // 실패 시 빈 문자열 → dispatcher가 "localhost"로 대체
	s.dispatcher = scheduler.NewCeleryDispatcher(redisAdapter, cfg.CeleryQueue, hostname)

	// 단계 (i): WorkflowService → RESTHandler
	workflowSvc := server.NewWorkflowService(pgStore, sm, logger)
	s.workflowSvc = workflowSvc

	restHandler := server.NewRESTHandler(workflowSvc, logger)
	s.restHandler = restHandler

	return s, nil
}

// Run 서버를 시작한다 — gRPC + REST dual listener (errgroup).
// ctx가 취소되거나 어느 한 리스너가 fatal error를 반환하면 graceful shutdown을 수행한다.
//
// @MX:ANCHOR: [AUTO] dual listener 진입점 — main.go + server_test.go 2곳에서 호출
// @MX:REASON: errgroup 기반 두 리스너의 생명주기를 단일 지점에서 관리
func (s *Server) Run(ctx context.Context) error {
	// 단계 (j): Auth chain 조합
	// s.recorder(*audit.Recorder)는 auth.auditRecorder 인터페이스(LogForbiddenEvent)를 구현하지 않음.
	// 현재는 nil 전달 (recorder=nil → 감사 기록 skip); S2에서 audit.Recorder에 LogForbiddenEvent 추가 예정.
	grpcServerOption := auth.BuildGRPCInterceptorChain(s.tokenValidator, nil, s.cfg.AuthEnabled)

	// gRPC 서버 생성 및 서비스 등록
	s.grpcServer = grpc.NewServer(grpcServerOption)
	// WorkflowService 등록 (REQ-SERVER-UBI-001-b 단계 j)
	proto.RegisterWorkflowServiceServer(s.grpcServer, s.workflowSvc)
	// 헬스체크 서버 등록 (REQ-SERVER-004-E3)
	grpc_health_v1.RegisterHealthServer(s.grpcServer, &grpcHealthServer{srv: s})

	// REST outer mux: /health, /ready는 auth chain 외부에 마운트 (AUTH-002 chain 진입 없음)
	outerMux := http.NewServeMux()
	outerMux.HandleFunc("GET /health", livenessHandler)
	outerMux.HandleFunc("GET /ready", readinessHandler(s))

	// /metrics: MetricsAuthMiddleware(독립 authn+authz) → MetricsHandler
	// BuildRESTChain 외부에서 독립적으로 마운트 — auth chain bypass (REQ-OBS-002)
	// @MX:NOTE: [AUTO] /metrics는 BuildRESTChain 외부 독립 마운트 — MetricsAuthMiddleware 전용 authn+authz 적용
	outerMux.Handle("GET /metrics",
		metrics.MetricsAuthMiddleware(s.tokenValidator, s.cfg.AuthEnabled)(
			metrics.MetricsHandler(),
		),
	)

	// 나머지 경로: auth chain으로 wrapping
	// recorder=nil: audit.Recorder가 auth.auditRecorder 인터페이스를 아직 구현하지 않음 (S2 TODO)
	outerMux.Handle("/", auth.BuildRESTChain(
		s.restHandler.Mux(),
		s.tokenValidator,
		nil,
		s.cfg.AuthEnabled,
	))

	// HTTP 서버 생성 (G112 Slowloris 방어: ReadHeaderTimeout 명시)
	s.httpServer = &http.Server{
		Addr:              s.cfg.RESTAddr,
		Handler:           outerMux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// gRPC 리스너 바인딩
	grpcLn, err := net.Listen("tcp", s.cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listener bind failed on %s: %w", s.cfg.GRPCAddr, err)
	}
	s.grpcAddr = grpcLn.Addr().String()

	// REST 리스너 바인딩
	restLn, err := net.Listen("tcp", s.cfg.RESTAddr)
	if err != nil {
		grpcLn.Close() //nolint:errcheck
		return fmt.Errorf("listener bind failed on %s: %w", s.cfg.RESTAddr, err)
	}
	s.restAddr = restLn.Addr().String()
	// httpServer.Addr를 실제 바인딩된 주소로 업데이트
	s.httpServer.Addr = s.restAddr

	// 부팅 시작 기록
	s.startedAt = time.Now()
	s.logger.Info("서버 리스너 바인딩 완료",
		zap.String("grpc_addr", s.grpcAddr),
		zap.String("rest_addr", s.restAddr),
	)

	// errgroup으로 두 리스너 병렬 실행
	g, gCtx := errgroup.WithContext(ctx)

	// gRPC goroutine
	g.Go(func() error {
		if err := s.grpcServer.Serve(grpcLn); err != nil {
			// gRPC 정상 종료는 nil 반환 (GracefulStop/Stop 이후)
			return fmt.Errorf("grpc serve: %w", err)
		}
		return nil
	})

	// REST goroutine
	g.Go(func() error {
		if err := s.httpServer.Serve(restLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http serve: %w", err)
		}
		return nil
	})

	// context 취소(SIGTERM/SIGINT) 또는 리스너 fatal error 감지 goroutine
	g.Go(func() error {
		<-gCtx.Done()
		// graceful shutdown 트리거 (S2에서 본격 구현; S1에서는 기본 종료)
		s.shutdown(context.Background(), "context_canceled")
		return nil
	})

	// SERVER_STARTUP audit row (두 리스너 바인딩 성공 후)
	s.recordAudit(context.Background(), audit.ActionServerStartup, map[string]any{
		"grpc_addr": s.grpcAddr,
		"rest_addr": s.restAddr,
	})

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

// shutdown graceful shutdown을 수행한다.
// sync.Once로 idempotency를 보장한다.
//
// @MX:WARN: [AUTO] goroutine 종료 race, idempotency 강제, force-kill 분기 — 변경 시 SIGTERM 처리 깨질 위험
// @MX:REASON: sync.Once + httpServer.Shutdown + grpcServer.GracefulStop 3 component race — 순서 변경 금지
func (s *Server) shutdown(ctx context.Context, reason string) {
	s.shutdownOnce.Do(func() {
		// shuttingDown 플래그 설정 (readiness probe에서 즉시 503 반환)
		s.mu.Lock()
		s.shuttingDown = true
		s.mu.Unlock()

		s.logger.Info("graceful shutdown 시작", zap.String("reason", reason))

		// SERVER_SHUTDOWN_INITIATED audit
		s.recordAudit(ctx, audit.ActionServerShutdownInitiated, map[string]any{
			"signal": reason,
		})

		shutdownTimeout := time.Duration(s.cfg.ShutdownTimeoutSeconds) * time.Second
		shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
		defer cancel()

		// (i) HTTP graceful shutdown
		if s.httpServer != nil {
			if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
				s.logger.Warn("HTTP shutdown 오류", zap.Error(err))
				if errors.Is(err, context.DeadlineExceeded) {
					s.logger.Warn("HTTP shutdown 타임아웃 — force close",
						zap.String("phase", "shutdown"),
						zap.String("reason", "timeout_force_kill"),
					)
					_ = s.httpServer.Close() //nolint:errcheck
				}
			}
		}

		// (ii) gRPC graceful stop (timeout 경쟁)
		if s.grpcServer != nil {
			grpcDone := make(chan struct{})
			go func() {
				s.grpcServer.GracefulStop()
				close(grpcDone)
			}()
			select {
			case <-grpcDone:
				// 정상 완료
			case <-shutdownCtx.Done():
				s.grpcServer.Stop()
			}
		}

		// (iii) Redis client close
		if s.redisClient != nil {
			if err := s.redisClient.Close(); err != nil {
				s.logger.Warn("Redis close 오류", zap.Error(err))
			}
		}

		// (iv) PgStore close
		if s.pgStore != nil {
			s.pgStore.Close()
		}

		// (v) OTel TracerProvider graceful shutdown
		// @MX:NOTE: [AUTO] tracerShutdown은 sdkTP.Shutdown — span buffer flush 후 종료
		if s.tracerShutdown != nil {
			if err := s.tracerShutdown(shutdownCtx); err != nil {
				s.logger.Warn("OTel TracerProvider shutdown 오류", zap.Error(err))
			}
		}

		// SERVER_SHUTDOWN_COMPLETED audit
		uptime := time.Since(s.startedAt).Seconds()
		exitReason := "graceful"
		if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
			exitReason = "force_kill_timeout"
		}
		if reason == "double_signal" {
			exitReason = "double_signal_force"
		}
		s.recordAudit(context.Background(), audit.ActionServerShutdownCompleted, map[string]any{
			"exit_reason":    exitReason,
			"uptime_seconds": uptime,
		})

		s.logger.Info("graceful shutdown 완료",
			zap.String("exit_reason", exitReason),
			zap.Float64("uptime_seconds", uptime),
		)
	})
}

// GRPCAddr 실제 바인딩된 gRPC 주소를 반환한다 (테스트에서 :0 사용 시 조회용).
func (s *Server) GRPCAddr() string { return s.grpcAddr }

// RESTAddr 실제 바인딩된 REST 주소를 반환한다 (테스트에서 :0 사용 시 조회용).
func (s *Server) RESTAddr() string { return s.restAddr }

// recordAudit audit_logs 테이블에 서버 이벤트를 기록한다.
// 레코더가 nil이거나 기록 실패해도 서버 동작에 영향 없음.
func (s *Server) recordAudit(_ context.Context, action audit.Action, details map[string]any) {
	if s.recorder == nil {
		return
	}
	// 현재 Recorder는 AuditTx 기반이므로 서버 lifecycle 이벤트는 zap 로그로 대체
	// (S2에서 직접 DB insert 구현 — REQ-SERVER-UBI-001-a)
	s.logger.Info("audit event",
		zap.String("action", string(action)),
		zap.Any("details", details),
	)
}
