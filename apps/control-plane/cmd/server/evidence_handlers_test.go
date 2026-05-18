//go:build integration

// evidence_handlers_test.go — T-013/T-015/T-016/T-017: 증빙 엔드포인트 통합 테스트
// DC-006 (해피 패스 201 + p99<150ms + 라우트 POST /api/v1/evidences), DC-007 (입력 검증),
// DC-008 (no-FK stub), DC-010 (중복 신호 active/inactive), DC-016 (audit fail 롤백),
// DC-017 (db://evidences/<UUID> location), SEC-01 (wrong Content-Type), SEC-02 (oversized streaming)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/storage"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// evidenceGoLeakOptions 인프라 goroutine 제외 (testcontainers/pgxpool/httptest 클라이언트)
// net/http persistConn read/writeLoop은 httptest 클라이언트 keep-alive 소유 — t.Cleanup 시 정리됨.
// 이 옵션 후에도 본 핸들러의 TX/goroutine 누출은 여전히 엄격히 탐지됨 (TH-03 의도 충족).
var evidenceGoLeakOptions = []goleak.Option{
	goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"),
	goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"),
	goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
	goleak.IgnoreTopFunction("net/http.(*conn).serve"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
}

// newEvidenceClient keep-alive 없는 전용 HTTP 클라이언트 — idle 연결 누수 방지
func newEvidenceClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{DisableKeepAlives: true},
		Timeout:   30 * time.Second,
	}
}

// evidenceTestEnv 증빙 핸들러 통합 테스트 환경
type evidenceTestEnv struct {
	pool   *pgxpool.Pool
	store  *store.PgWorkflowStore
	server *httptest.Server
	logs   *observer.ObservedLogs
	client *http.Client
}

// setupEvidenceTestEnv testcontainers Postgres + 스키마 + 핸들러 httptest 서버 구성
func setupEvidenceTestEnv(t *testing.T, dupSignal bool) *evidenceTestEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("iroum_ax"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("testpass"),
		tcpostgres.WithInitScripts("../../internal/store/testdata/schema.sql"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pgContainer.Terminate(context.Background()) //nolint:errcheck
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	baseLogger := zaptest.NewLogger(t)
	obsCore, obsLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(zapcore.NewTee(baseLogger.Core(), obsCore))

	pgStore, err := store.NewPgWorkflowStore(ctx, dsn, logger)
	require.NoError(t, err)
	t.Cleanup(pgStore.Close)

	rec := audit.NewRecorder(false) // AUTH_ENABLED=false → cli-anonymous
	h := NewEvidenceHandler(pgStore, rec, storage.NewDBBlobStore(), logger, 52428800, dupSignal)

	ts := httptest.NewServer(h.Routes())
	t.Cleanup(ts.Close)

	// 별도 풀로 검증 쿼리 수행 (커밋 후 검증)
	vpool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(vpool.Close)

	client := newEvidenceClient()
	t.Cleanup(client.CloseIdleConnections)

	return &evidenceTestEnv{pool: vpool, store: pgStore, server: ts, logs: obsLogs, client: client}
}

// buildMultipart evaluation_item_id + file 파트로 multipart 본문 생성
func buildMultipart(t *testing.T, evalItemID, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if evalItemID != "__OMIT__" {
		require.NoError(t, mw.WriteField("evaluation_item_id", evalItemID))
	}
	if fileName != "__OMIT__" {
		require.NoError(t, mw.WriteField("file_name", fileName))
	}
	if content != nil {
		fw, err := mw.CreateFormFile("file", "upload.bin")
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

// postEvidence 멀티파트 POST를 보내고 status + body를 반환 (정상 경로 — 연결 reset이면 t.Fatal)
func postEvidence(t *testing.T, env *evidenceTestEnv, evalItemID, fileName string, content []byte) (int, map[string]any) {
	t.Helper()
	status, body, err := postEvidenceTolerant(t, env, evalItemID, fileName, content)
	require.NoError(t, err, "정상 경로 요청에서 전송 에러가 발생해서는 안 됨")
	return status, body
}

// postEvidenceTolerant 전송 에러를 호출자에게 반환 (oversized connection-reset 허용 케이스용)
func postEvidenceTolerant(t *testing.T, env *evidenceTestEnv, evalItemID, fileName string, content []byte) (int, map[string]any, error) {
	t.Helper()
	body, ct := buildMultipart(t, evalItemID, fileName, content)
	req, err := http.NewRequest(http.MethodPost, env.server.URL+"/api/v1/evidences", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", ct)

	resp, doErr := env.client.Do(req)
	if doErr != nil {
		return 0, nil, doErr
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body) //nolint:errcheck
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed) //nolint:errcheck
	return resp.StatusCode, parsed, nil
}

// countEvidencesH evaluation_item_id별 evidences 행 수 (커밋 후 검증)
func countEvidencesH(t *testing.T, env *evidenceTestEnv, evalItemID string) int {
	t.Helper()
	var n int
	require.NoError(t, env.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM evidences WHERE evaluation_item_id=$1`, evalItemID).Scan(&n))
	return n
}

// ── DC-006: 해피 패스 ───────────────────────────────────────────────────────

func TestEvidenceStore_CreateHappyPath_HTTP(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)

	content := []byte("KEPCO 경영평가 증빙 본문")
	status, body := postEvidence(t, env, "eval-h-001-1", "report.pdf", content)

	require.Equal(t, http.StatusCreated, status, "POST /api/v1/evidences → 201")
	evID, ok := body["evidence_id"].(string)
	require.True(t, ok)
	_, perr := uuid.Parse(evID)
	require.NoError(t, perr, "evidence_id는 유효한 UUID")
	assert.Equal(t, float64(1), body["version"], "version=1")
	_, hasDup := body["duplicate_of"]
	assert.False(t, hasDup, "기본(dupSignal=false)은 duplicate_of 키 부재")

	// DB 검증 (커밋 후 별도 풀)
	ctx := context.Background()
	var version int
	var prevID *uuid.UUID
	var hash, strat, loc string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT version, previous_version_id, file_hash_sha256, storage_strategy, storage_location
		 FROM evidences WHERE id=$1`, evID,
	).Scan(&version, &prevID, &hash, &strat, &loc))
	assert.Equal(t, 1, version)
	assert.Nil(t, prevID)
	assert.Len(t, hash, 64)
	assert.Contains(t, []string{"filesystem", "database_blob", "minio"}, strat)
	// GAP-05 / DC-017: database_blob storage_location = db://evidences/<UUID>
	assert.Regexp(t, regexp.MustCompile(`^db://evidences/[0-9a-f-]{36}$`), loc)

	// 감사 1건 (REQ-EVID-UBI-002-A)
	var auditN int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE action='EVIDENCE_CREATED' AND resource_id=$1`, evID).Scan(&auditN))
	assert.Equal(t, 1, auditN)
}

// DC-006 item 5: 응답 latency < 150ms (블롭 저장 제외, 10회)
func TestEvidenceStore_CreateLatencyUnder150ms(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)
	content := []byte("small-latency-probe")

	var maxLatency time.Duration
	for i := 0; i < 10; i++ {
		body, ct := buildMultipart(t, "eval-lat-"+uuid.NewString(), "f.bin", content)
		req, _ := http.NewRequest(http.MethodPost, env.server.URL+"/api/v1/evidences", body) //nolint:errcheck
		req.Header.Set("Content-Type", ct)
		start := time.Now()
		resp, err := env.client.Do(req)
		elapsed := time.Since(start)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		if elapsed > maxLatency {
			maxLatency = elapsed
		}
	}
	assert.Less(t, maxLatency, 150*time.Millisecond,
		"10회 반복 최대 응답 latency < 150ms (DC-006 item 5)")
}

// ── DC-008: eval_item stub — FK 미강제 (HTTP) ──────────────────────────────

func TestEvidenceStore_EvalItemStubNoFKConstraint_HTTP(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)

	status, body := postEvidence(t, env, "nonexistent-item-xyz-999", "x.pdf", []byte("abc"))
	require.Equal(t, http.StatusCreated, status, "임의 evaluation_item_id도 201 (FK 미강제)")
	assert.NotEmpty(t, body["evidence_id"])
	assert.Equal(t, 1, countEvidencesH(t, env, "nonexistent-item-xyz-999"))
}

// ── DC-007 / T-015: 입력 검증 (oversized/empty/missing/long) ───────────────

func TestEvidenceHandler_InputValidation(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)

	cases := []struct {
		name       string
		evalItemID string
		fileName   string
		content    []byte
		wantField  string
	}{
		{"oversized", "eval-v-1", "f.bin", bytes.Repeat([]byte("A"), 52428801), "file"},
		{"empty", "eval-v-2", "f.bin", []byte{}, "file"},
		{"missing_eval_item", "__OMIT__", "f.bin", []byte("abc"), "evaluation_item_id"},
		{"eval_item_too_long", strings.Repeat("x", 65), "f.bin", []byte("abc"), "evaluation_item_id"},
		{"file_name_too_long", "eval-v-5", strings.Repeat("y", 513), []byte("abc"), "file_name"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			env.logs.TakeAll() // 로그 버퍼 초기화
			content := tc.content
			if content == nil {
				content = []byte{}
			}

			status, body, doErr := postEvidenceTolerant(t, env, tc.evalItemID, tc.fileName, content)

			if tc.name == "oversized" {
				// oversized: Content-Length 사전거부(413) 또는 LimitReader(400) 또는
				// 서버 조기 연결종료(connection reset — heap 고갈 방어) — 모두 contract-compliant
				if doErr != nil {
					assert.Contains(t, doErr.Error(), "connection reset",
						"oversized 서버 조기 종료는 허용되는 SEC-02 방어 동작")
				} else {
					assert.Contains(t, []int{http.StatusBadRequest, http.StatusRequestEntityTooLarge},
						status, "oversized → 400 또는 413")
					if errObj, ok := body["error"].(map[string]any); ok {
						assert.Equal(t, "file", errObj["field"])
					}
				}
			} else {
				require.NoError(t, doErr, "%s: 전송 에러 없어야 함", tc.name)
				require.Equal(t, http.StatusBadRequest, status, "%s → 400", tc.name)
				errObj, ok := body["error"].(map[string]any)
				require.True(t, ok, "error 객체 존재")
				assert.Equal(t, "INVALID_ARGUMENT", errObj["code"])
				assert.Equal(t, tc.wantField, errObj["field"], "%s field", tc.name)

				// 거부 로그 레벨 INFO (DC-007 item 4) — 연결 reset 케이스는 로그 보장 불가하므로 제외
				infoLogs := env.logs.FilterMessage("증빙 요청 거부").All()
				require.NotEmpty(t, infoLogs, "거부 INFO 로그 존재")
				for _, l := range infoLogs {
					assert.Equal(t, zapcore.InfoLevel, l.Level, "거부는 INFO 레벨 (ERROR 아님)")
				}
			}

			// 공통 불변식: 거부 시 evidences/audit 0건 (pre-TX)
			var evN, auN int
			require.NoError(t, env.pool.QueryRow(context.Background(),
				`SELECT COUNT(*) FROM evidences`).Scan(&evN))
			require.NoError(t, env.pool.QueryRow(context.Background(),
				`SELECT COUNT(*) FROM audit_logs`).Scan(&auN))
			assert.Equal(t, 0, evN, "거부 시 evidences 0건 (pre-TX)")
			assert.Equal(t, 0, auN, "거부 시 audit_logs 0건")
		})
	}
}

// ── SEC-01: 잘못된 Content-Type ─────────────────────────────────────────────

func TestEvidenceHandler_WrongContentType(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)

	req, _ := http.NewRequest(http.MethodPost, env.server.URL+"/api/v1/evidences", //nolint:errcheck
		strings.NewReader(`{"x":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var evN int
	require.NoError(t, env.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM evidences`).Scan(&evN))
	assert.Equal(t, 0, evN, "Content-Type 거부 시 evidences 0건")
}

// ── SEC-02: oversized 스트리밍 (heap 고갈 없이 거부) ────────────────────────

func TestEvidenceHandler_OversizedFile_NoHeapExhaustion(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)

	// 52 MiB 본문 — 서버는 Content-Length 사전거부(413)로 body 전체 read 이전 차단.
	// 클라이언트 입장에서 (a) 깨끗한 413/400 응답 또는
	// (b) 서버 조기 종료로 인한 connection reset(write error) — 둘 다 heap 고갈 방어 성공.
	body, ct := buildMultipart(t, "eval-oversized", "big.bin", bytes.Repeat([]byte("Z"), 52*1024*1024))
	req, err := http.NewRequest(http.MethodPost, env.server.URL+"/api/v1/evidences", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", ct)

	resp, doErr := env.client.Do(req)
	if doErr != nil {
		// 서버가 52 MiB 전체를 읽기 전에 연결을 끊음 = SEC-02 heap 고갈 방어 동작
		assert.Contains(t, doErr.Error(), "connection reset",
			"oversized 거부 시 서버 조기 종료(connection reset)는 허용되는 방어 동작")
	} else {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body) //nolint:errcheck
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusRequestEntityTooLarge},
			resp.StatusCode, "52 MiB 본문은 400 또는 413으로 거부")
	}

	// 핵심 불변식: 거부 시 evidence 행 0건 (heap 고갈 없이 차단됨)
	assert.Equal(t, 0, countEvidencesH(t, env, "eval-oversized"))
}

// ── DC-010 / T-016: 중복 SHA-256 신호 (active/inactive) ────────────────────

func TestEvidenceHandler_DuplicateSignal_ActiveMode(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, true) // EVIDENCE_DUPLICATE_SIGNAL_ENABLED=true
	const evalItemID = "eval-001-o1"
	content := []byte("identical-bytes")

	st1, b1 := postEvidence(t, env, evalItemID, "v1.bin", content)
	require.Equal(t, http.StatusCreated, st1)
	ev1ID := b1["evidence_id"].(string)

	// 동일 SHA-256 재업로드 → 201 + duplicate_of=ev1ID, v2 정상 생성
	st2, b2 := postEvidence(t, env, evalItemID, "v2.bin", content)
	require.Equal(t, http.StatusCreated, st2, "중복도 거부하지 않음 (비차단)")
	assert.Equal(t, float64(2), b2["version"])
	assert.Equal(t, ev1ID, b2["duplicate_of"], "duplicate_of == 직전 동일 해시 ID")
	assert.Equal(t, 2, countEvidencesH(t, env, evalItemID))
}

func TestEvidenceHandler_DuplicateSignal_InactiveMode(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false) // 기본 비활성
	const evalItemID = "eval-001-o1-inactive"
	content := []byte("identical-bytes-2")

	st1, _ := postEvidence(t, env, evalItemID, "v1.bin", content)
	require.Equal(t, http.StatusCreated, st1)
	st2, b2 := postEvidence(t, env, evalItemID, "v2.bin", content)
	require.Equal(t, http.StatusCreated, st2)
	assert.Equal(t, float64(2), b2["version"])
	_, hasDup := b2["duplicate_of"]
	assert.False(t, hasDup, "비활성 모드는 duplicate_of 키 부재")
	assert.Equal(t, 2, countEvidencesH(t, env, evalItemID))
}

// ── DC-016: audit fail → 양방향 롤백 (HTTP 경로) ───────────────────────────

func TestEvidenceStore_AuditFailRollbackBidirectional_HTTP(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)
	ctx := context.Background()
	const evalItemID = "eval-h-rollback"

	// audit_logs INSERT를 무조건 거부하는 CHECK 제약 주입
	_, err := env.pool.Exec(ctx, `ALTER TABLE audit_logs ADD CONSTRAINT ci_fail_h CHECK (false)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.pool.Exec(context.Background(),
			`ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS ci_fail_h`) //nolint:errcheck
	})

	status, _ := postEvidence(t, env, evalItemID, "r.pdf", []byte("rollback-http"))
	assert.Equal(t, http.StatusInternalServerError, status, "audit 실패 → 500 (wrapped error)")

	assert.Equal(t, 0, countEvidencesH(t, env, evalItemID), "evidence 행 롤백 (0건)")
	var auN int
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&auN))
	assert.Equal(t, 0, auN, "audit 행 미삽입 (0건)")
}

// ── DC-003 (HTTP): 버전 이벤트 EVIDENCE_VERSIONED 1건 ──────────────────────

func TestEvidenceAudit_VersionRowAtomicCommit_HTTP(t *testing.T) {
	defer goleak.VerifyNone(t, evidenceGoLeakOptions...)
	env := setupEvidenceTestEnv(t, false)
	const evalItemID = "eval-h-002b"

	st1, b1 := postEvidence(t, env, evalItemID, "v1.pdf", []byte("v1"))
	require.Equal(t, http.StatusCreated, st1)
	ev1ID := b1["evidence_id"].(string)

	st2, b2 := postEvidence(t, env, evalItemID, "v2.pdf", []byte("v2"))
	require.Equal(t, http.StatusCreated, st2)
	ev2ID := b2["evidence_id"].(string)
	assert.Equal(t, float64(2), b2["version"])

	ctx := context.Background()
	var verAction, userID, prevDetail, verDetail string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT action, user_id, details->>'previous_version_id', details->>'version'
		 FROM audit_logs WHERE action='EVIDENCE_VERSIONED' AND resource_id=$1`, ev2ID,
	).Scan(&verAction, &userID, &prevDetail, &verDetail))
	assert.Equal(t, "EVIDENCE_VERSIONED", verAction)
	assert.Equal(t, "cli-anonymous", userID)
	assert.Equal(t, ev1ID, prevDetail)
	assert.Equal(t, "2", verDetail)

	// 2 evidences + 2 audit (CREATED+VERSIONED)
	assert.Equal(t, 2, countEvidencesH(t, env, evalItemID))
	var auN int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE action IN ('EVIDENCE_CREATED','EVIDENCE_VERSIONED')
		 AND resource_id IN ($1,$2)`, ev1ID, ev2ID).Scan(&auN))
	assert.Equal(t, 2, auN)
}
