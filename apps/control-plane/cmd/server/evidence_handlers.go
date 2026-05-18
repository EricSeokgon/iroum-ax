// evidence_handlers.go — 증빙 생성/버전 REST 엔드포인트 (SPEC-AX-EVID-001)
// 라우트: POST /api/v1/evidences (GAP-01, contract.md DC-006)
// 흐름: Content-Type 검증(SEC-01) → Content-Length 사전 거부(SEC-02.1) →
//
//	multipart 단일 패스 스트리밍 SHA-256(SEC-02.2/3, F1) → 필드 검증(SEC-05) →
//	BeginEvidenceTx → defer Rollback(SEC-07) → blob 논리 location(TX 스코프 내, F2) →
//	버전 결정 → InsertEvidence(+file_content) → RecordEvidence{Created|Versioned} →
//	Commit → 201 {evidence_id, version}
//
// database_blob 전략: blob bytes는 동일 pgx TX의 file_content BYTEA에 저장 (strategy.md §2.6.5)
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/storage"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

// 핸들러 사전 검증 상수 (SEC-05 — DB ERROR 22001 누출 방지, DDL VARCHAR 길이 정합)
const (
	maxEvalItemIDLen = 64  // evaluation_item_id VARCHAR(64)
	maxFileNameLen   = 512 // file_name VARCHAR(512)
)

// errEvidenceOversized file 파트가 maxFileBytes를 초과 (SEC-02 단일 패스 거부 센티넬)
var errEvidenceOversized = errors.New("evidence file exceeds maximum allowed size")

// evidenceRecorder Recorder 의존 인터페이스 — audit.Recorder의 증빙 메서드 부분집합
// (테스트 격리 + R-EVID-001 store 미import 정합)
type evidenceRecorder interface {
	RecordEvidenceCreated(ctx context.Context, tx audit.AuditTx, evidenceID, evalItemID, fileHashSHA256 string, version int, userID string) error
	RecordEvidenceVersioned(ctx context.Context, tx audit.AuditTx, evidenceID, evalItemID, fileHashSHA256 string, version int, previousVersionID, userID string) error
}

// EvidenceHandler 증빙 생성/버전 엔드포인트 핸들러
// 필드 순서: 인터페이스/포인터(8B) 우선 (fieldalignment 최적화)
//
// @MX:ANCHOR: [AUTO] 증빙 REST 진입점 — 핸들러 테스트 + 서버 마운트 + 통합 테스트 3곳에서 사용
// @MX:REASON: 증빙 생성/버전 단일 HTTP 계약 (DC-006~DC-010, GAP-01 POST /api/v1/evidences)
type EvidenceHandler struct {
	store        store.EvidenceStore
	recorder     evidenceRecorder
	blobStore    storage.EvidenceBlobStore
	logger       *zap.Logger
	maxFileBytes int64
	dupSignal    bool
}

// NewEvidenceHandler 증빙 핸들러를 생성
// maxFileBytes: EVIDENCE_MAX_FILE_BYTES, dupSignal: EVIDENCE_DUPLICATE_SIGNAL_ENABLED
func NewEvidenceHandler(
	st store.EvidenceStore,
	rec evidenceRecorder,
	blob storage.EvidenceBlobStore,
	logger *zap.Logger,
	maxFileBytes int64,
	dupSignal bool,
) *EvidenceHandler {
	return &EvidenceHandler{
		store:        st,
		recorder:     rec,
		blobStore:    blob,
		logger:       logger,
		maxFileBytes: maxFileBytes,
		dupSignal:    dupSignal,
	}
}

// Routes 증빙 라우트를 등록한 http.Handler 반환 (GAP-01: POST /api/v1/evidences)
func (h *EvidenceHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/evidences", h.handleCreateEvidence)
	return mux
}

// evidenceErrorBody 400 등 에러 응답 — {"error":{"code","message","field"}}
type evidenceErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

// evidenceCreatedBody 201 성공 응답 — {evidence_id, version, duplicate_of?}
type evidenceCreatedBody struct {
	EvidenceID  string `json:"evidence_id"`
	DuplicateOf string `json:"duplicate_of,omitempty"`
	Version     int    `json:"version"`
}

// writeEvidenceJSON Content-Type 설정 후 JSON 직렬화
func writeEvidenceJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // 헤더 전송 후라 로깅 불가
}

// writeEvidenceErr 400 등 표준 에러 본문 + INFO 로그 (DC-007 item 4 — 거부는 INFO 레벨)
func (h *EvidenceHandler) writeEvidenceErr(w http.ResponseWriter, code int, errCode, msg, field string) {
	var body evidenceErrorBody
	body.Error.Code = errCode
	body.Error.Message = msg
	body.Error.Field = field
	h.logger.Info("증빙 요청 거부",
		zap.Int("http_status", code),
		zap.String("error_code", errCode),
		zap.String("field", field),
	)
	writeEvidenceJSON(w, code, body)
}

// parseAndHashMultipart file 파트를 단일 패스로 스트리밍하며 SHA-256을 계산한다 (F1, SEC-02 item 3).
//
// io.Copy(io.MultiWriter(&buf, hasher), io.LimitReader(r, maxBytes+1)) 패턴:
// 경계 제한 읽기와 해싱이 동일 패스에서 일어나며, 전체 본문을 먼저 ReadAll한 뒤
// 해싱하지 않는다. 소비량 n == maxBytes+1 이면 oversized 로 errEvidenceOversized 반환
// (잔여 바이트는 버퍼링하지 않음 — heap 고갈 방어).
//
// 반환: (버퍼된 바이트, sha256 sum, 소비 바이트 수, error)
func parseAndHashMultipart(r io.Reader, maxBytes int64) ([]byte, []byte, int64, error) {
	hasher := sha256.New()
	var buf bytes.Buffer
	// LimitReader 경계는 maxBytes+1 — 초과 1바이트로 oversized 판정 (잔여는 읽지 않음)
	n, err := io.Copy(io.MultiWriter(&buf, hasher), io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, nil, n, err
	}
	if n > maxBytes {
		return nil, nil, n, errEvidenceOversized
	}
	return buf.Bytes(), hasher.Sum(nil), n, nil
}

// evidenceFormParts multipart 파싱 결과 (file 바이트는 스트리밍 해시 후 보관)
// 필드 순서: string(16B) → slice(24B) → bool(1B) (fieldalignment govet 정렬)
type evidenceFormParts struct {
	evalItemID string
	fileName   string
	fileHash   string
	fileBytes  []byte
	gotFile    bool
}

// parseMultipartForm multipart 스트림을 파트 단위로 처리한다 (전체 버퍼링 금지).
// file 파트는 parseAndHashMultipart로 단일 패스 스트리밍 해싱한다 (F1, SEC-02 item 3).
func (h *EvidenceHandler) parseMultipartForm(r *http.Request) (*evidenceFormParts, int, string) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, http.StatusBadRequest, "multipart 파싱 실패: " + err.Error()
	}

	parts := &evidenceFormParts{}
	for {
		part, perr := mr.NextPart()
		if errors.Is(perr, io.EOF) {
			break
		}
		if perr != nil {
			return nil, http.StatusBadRequest, "multipart 파트 읽기 실패"
		}
		switch part.FormName() {
		case "evaluation_item_id":
			b, _ := io.ReadAll(io.LimitReader(part, maxEvalItemIDLen+1)) //nolint:errcheck
			parts.evalItemID = strings.TrimSpace(string(b))
		case "file_name":
			b, _ := io.ReadAll(io.LimitReader(part, maxFileNameLen+1)) //nolint:errcheck
			parts.fileName = strings.TrimSpace(string(b))
		case "file":
			if parts.fileName == "" {
				parts.fileName = part.FileName()
			}
			// ── SEC-02.2/3 (F1): 단일 패스 스트리밍 — ReadAll-후-해싱 금지 ──
			fileBytes, sum, _, herr := parseAndHashMultipart(part, h.maxFileBytes)
			if herr != nil {
				_ = part.Close() //nolint:errcheck
				if errors.Is(herr, errEvidenceOversized) {
					return nil, http.StatusBadRequest, "file exceeds maximum allowed size"
				}
				return nil, http.StatusBadRequest, "file 읽기 실패"
			}
			parts.fileBytes = fileBytes
			parts.fileHash = hex.EncodeToString(sum)
			parts.gotFile = true
		}
		_ = part.Close() //nolint:errcheck
	}
	return parts, 0, ""
}

// validateEvidenceParts pre-TX 입력 검증 (SEC-05 / T-015 — TX 미진입, row 0건 보장).
// 반환: (errField, errMsg) — errField=="" 이면 검증 통과
func validateEvidenceParts(p *evidenceFormParts) (string, string) {
	switch {
	case p.evalItemID == "":
		return "evaluation_item_id", "evaluation_item_id는 필수입니다"
	case len(p.evalItemID) > maxEvalItemIDLen:
		return "evaluation_item_id", "evaluation_item_id가 64자를 초과합니다"
	case len(p.fileName) > maxFileNameLen:
		return "file_name", "file_name이 512자를 초과합니다"
	case !p.gotFile || len(p.fileBytes) == 0:
		return "file", "file은 필수이며 비어 있을 수 없습니다"
	}
	return "", ""
}

// versionResolution 버전 결정 결과 (직전 행 + 새 버전 번호 + 중복 신호)
type versionResolution struct {
	prevID  *uuid.UUID
	dupOf   string
	version int
}

// resolveVersion SELECT FOR UPDATE로 직전 버전을 조회하고 새 버전 번호를 결정한다.
// 기존 행이 없으면 version=1, 있으면 latest.Version+1 + previous_version_id 체이닝.
// dupSignal 활성 + 동일 SHA-256이면 dupOf에 직전 ID 기록 (비차단 — T-016).
func (h *EvidenceHandler) resolveVersion(ctx context.Context, tx store.EvidenceTx, evalItemID, fileHash string) (*versionResolution, error) {
	latest, err := tx.GetLatestVersionByEvalItem(ctx, evalItemID)
	if err != nil {
		return nil, err
	}
	res := &versionResolution{version: 1}
	if latest != nil {
		id := latest.ID
		res.prevID = &id
		res.version = latest.Version + 1
		if h.dupSignal && latest.FileHashSHA256 == fileHash {
			res.dupOf = latest.ID.String()
		}
	}
	return res, nil
}

// persistEvidenceTx 동일 EvidenceTx 내에서 blob 논리 location 생성(F2) → InsertEvidence →
// (버전 시) MarkSuperseded → RecordEvidence{Created|Versioned} → Commit 을 수행한다.
// blob 논리 location 생성은 TX 스코프 내에서 발생한다 (F2 — database_blob에서
// dbBlobStore.Put은 side-effect-free이며 bytes는 InsertEvidence(file_content) 경유,
// strategy.md §2.6.5 option 6a; 향후 filesystem 구현도 TX 경계 내에 위치하도록 보장).
//
// 반환: 생성된 증빙 ID, error
func (h *EvidenceHandler) persistEvidenceTx(
	ctx context.Context, tx store.EvidenceTx,
	p *evidenceFormParts, vr *versionResolution,
) (uuid.UUID, error) {
	const contentType = "application/octet-stream"

	// F2: blob 논리 location 생성을 TX 스코프 내에서 수행 (database_blob: side-effect-free).
	// 어떤 코드 경로에서도 evidence TX 밖에서 blob을 영속하지 않는다 (REQ-EVID-UBI-002).
	newID := uuid.New()
	storageLocation, blobErr := h.blobStore.Put(ctx, newID.String(), bytes.NewReader(nil))
	if blobErr != nil {
		return uuid.Nil, blobErr
	}

	insertedID, err := tx.InsertEvidence(ctx,
		p.evalItemID, p.fileName, contentType,
		int64(len(p.fileBytes)), p.fileHash,
		"database_blob", storageLocation,
		nil, p.fileBytes, vr.prevID,
	)
	if err != nil {
		return uuid.Nil, err
	}

	// 버전인 경우 직전 행 SUPERSEDED (store 계층 소유 — GAP-04)
	if vr.prevID != nil {
		if mErr := tx.MarkSuperseded(ctx, *vr.prevID); mErr != nil {
			return uuid.Nil, mErr
		}
	}

	// RecordEvidence{Created|Versioned} — 동일 TX 감사 원자성
	if vr.version == 1 {
		err = h.recorder.RecordEvidenceCreated(ctx, tx, insertedID.String(), p.evalItemID, p.fileHash, vr.version, "")
	} else {
		err = h.recorder.RecordEvidenceVersioned(ctx, tx, insertedID.String(), p.evalItemID, p.fileHash, vr.version, vr.prevID.String(), "")
	}
	if err != nil {
		return uuid.Nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return insertedID, nil
}

// handleCreateEvidence POST /api/v1/evidences — 증빙 생성/버전 처리 (오케스트레이션 전용)
//
// @MX:WARN: [AUTO] BeginEvidenceTx 이후 Commit 전 panic/early-return 시 orphan 증빙 행 누출
// @MX:REASON: defer Rollback이 BeginEvidenceTx 직후 즉시 등록되어야 함 — 순서 변경 금지 (SEC-07, research.md §8 Risk 3)
func (h *EvidenceHandler) handleCreateEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// ── SEC-01: Content-Type 검증 (body read 이전) ──────────────────────────
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		h.writeEvidenceErr(w, http.StatusBadRequest, "INVALID_ARGUMENT",
			"Content-Type must be multipart/form-data", "")
		return
	}
	// ── SEC-02.1: Content-Length 사전 거부 (body read 이전) ─────────────────
	if r.ContentLength > 0 && r.ContentLength > h.maxFileBytes {
		h.writeEvidenceErr(w, http.StatusRequestEntityTooLarge, "INVALID_ARGUMENT",
			"file exceeds maximum allowed size", "file")
		return
	}

	// ── multipart 단일 패스 스트리밍 SHA-256 (F1, SEC-02.2/3) ───────────────
	parts, code, msg := h.parseMultipartForm(r)
	if code != 0 {
		field := ""
		if strings.Contains(msg, "exceeds maximum") {
			field = "file"
		}
		h.writeEvidenceErr(w, code, "INVALID_ARGUMENT", msg, field)
		return
	}

	// ── SEC-05 / T-015: pre-TX 입력 검증 (TX 미진입, row 0건) ──────────────
	if errField, errMsg := validateEvidenceParts(parts); errField != "" {
		h.writeEvidenceErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", errMsg, errField)
		return
	}
	if parts.fileName == "" {
		parts.fileName = "evidence.bin"
	}

	// ── TX orchestration: BeginEvidenceTx → defer Rollback (SEC-07) ────────
	tx, err := h.store.BeginEvidenceTx(ctx)
	if err != nil {
		h.logger.Error("BeginEvidenceTx 실패", zap.Error(err))
		h.writeEvidenceErr(w, http.StatusInternalServerError, "INTERNAL", "트랜잭션 시작 실패", "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx) //nolint:errcheck // BeginEvidenceTx 직후 즉시 등록 (SEC-07)
		}
	}()

	// ── 버전 결정 (SELECT FOR UPDATE 직렬화) ───────────────────────────────
	vr, err := h.resolveVersion(ctx, tx, parts.evalItemID, parts.fileHash)
	if err != nil {
		h.logger.Error("버전 결정 실패", zap.Error(err))
		h.writeEvidenceErr(w, http.StatusInternalServerError, "INTERNAL", "버전 조회 실패", "")
		return
	}

	// ── 영속화: blob location(TX 스코프, F2) → Insert → Supersede → 감사 → Commit ──
	insertedID, err := h.persistEvidenceTx(ctx, tx, parts, vr)
	if err != nil {
		h.logger.Error("증빙 영속화 실패 — 양방향 롤백", zap.Error(err))
		h.writeEvidenceErr(w, http.StatusInternalServerError, "INTERNAL", "증빙 저장 실패", "")
		return
	}
	committed = true

	resp := evidenceCreatedBody{EvidenceID: insertedID.String(), Version: vr.version}
	if vr.dupOf != "" {
		resp.DuplicateOf = vr.dupOf
	}
	writeEvidenceJSON(w, http.StatusCreated, resp)
}
