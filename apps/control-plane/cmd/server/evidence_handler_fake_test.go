//go:build integration

// evidence_handler_fake_test.go — T-019 커버리지 보강: 핸들러 에러 분기 (fake 기반, DB 불요)
// handleCreateEvidence의 BeginEvidenceTx 실패 / InsertEvidence 실패 / MarkSuperseded 실패 /
// 버전 결정 조회 실패 / Commit 실패 / RecordEvidence 실패 분기를 fault 주입으로 커버.
package main

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ircp/iroum-ax/apps/control-plane/internal/audit"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/storage"
	"github.com/ircp/iroum-ax/apps/control-plane/internal/store"
)

var errFakeEvidence = errors.New("fake evidence failure")

// fakeEvidenceStore 장애 주입 가능한 EvidenceStore
type fakeEvidenceStore struct {
	failBegin bool
	tx        *fakeEvidenceTx
}

func (s *fakeEvidenceStore) BeginEvidenceTx(_ context.Context) (store.EvidenceTx, error) {
	if s.failBegin {
		return nil, errFakeEvidence
	}
	return s.tx, nil
}

// fakeEvidenceTx 단계별 장애 주입 EvidenceTx
type fakeEvidenceTx struct {
	latest          *store.Evidence
	failGetLatest   bool
	failInsert      bool
	failSupersede   bool
	failCommit      bool
	rollbackCalled  bool
}

func (t *fakeEvidenceTx) InsertEvidence(_ context.Context, _, _, _ string, _ int64, _, _, _ string, _ map[string]string, _ []byte, _ *uuid.UUID) (uuid.UUID, error) {
	if t.failInsert {
		return uuid.Nil, errFakeEvidence
	}
	return uuid.New(), nil
}
func (t *fakeEvidenceTx) GetEvidenceByID(_ context.Context, _ uuid.UUID) (*store.Evidence, error) {
	return nil, errFakeEvidence
}
func (t *fakeEvidenceTx) GetLatestVersionByEvalItem(_ context.Context, _ string) (*store.Evidence, error) {
	if t.failGetLatest {
		return nil, errFakeEvidence
	}
	return t.latest, nil
}
func (t *fakeEvidenceTx) ListEvidenceByEvalItem(_ context.Context, _ string) ([]*store.Evidence, error) {
	return nil, nil
}
func (t *fakeEvidenceTx) MarkSuperseded(_ context.Context, _ uuid.UUID) error {
	if t.failSupersede {
		return errFakeEvidence
	}
	return nil
}
func (t *fakeEvidenceTx) InsertAuditLog(_ context.Context, _ *audit.Event) error { return nil }
func (t *fakeEvidenceTx) Commit(_ context.Context) error {
	if t.failCommit {
		return errFakeEvidence
	}
	return nil
}
func (t *fakeEvidenceTx) Rollback(_ context.Context) error { t.rollbackCalled = true; return nil }

// fakeRecorder 감사 기록 장애 주입
type fakeRecorder struct{ failRecord bool }

func (r *fakeRecorder) RecordEvidenceCreated(_ context.Context, _ audit.AuditTx, _, _, _ string, _ int, _ string) error {
	if r.failRecord {
		return errFakeEvidence
	}
	return nil
}
func (r *fakeRecorder) RecordEvidenceVersioned(_ context.Context, _ audit.AuditTx, _, _, _ string, _ int, _, _ string) error {
	if r.failRecord {
		return errFakeEvidence
	}
	return nil
}

func fakeMultipartReq(t *testing.T, url, evalItemID string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("evaluation_item_id", evalItemID))
	fw, err := mw.CreateFormFile("file", "f.bin")
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func runFakeHandler(t *testing.T, st store.EvidenceStore, rec evidenceRecorder, evalItemID string, content []byte) int {
	t.Helper()
	h := NewEvidenceHandler(st, rec, storage.NewDBBlobStore(), zap.NewNop(), 52428800, false)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, fakeMultipartReq(t, "/api/v1/evidences", evalItemID, content))
	return rr.Code
}

// TestEvidenceHandler_FaultBranches 핸들러 에러 분기 전수 커버
func TestEvidenceHandler_FaultBranches(t *testing.T) {
	t.Run("begin_tx_fail", func(t *testing.T) {
		code := runFakeHandler(t, &fakeEvidenceStore{failBegin: true}, &fakeRecorder{}, "ei", []byte("x"))
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("get_latest_fail", func(t *testing.T) {
		st := &fakeEvidenceStore{tx: &fakeEvidenceTx{failGetLatest: true}}
		code := runFakeHandler(t, st, &fakeRecorder{}, "ei", []byte("x"))
		assert.Equal(t, http.StatusInternalServerError, code)
		assert.True(t, st.tx.rollbackCalled, "실패 시 deferred Rollback 호출 (SEC-07)")
	})
	t.Run("insert_fail", func(t *testing.T) {
		st := &fakeEvidenceStore{tx: &fakeEvidenceTx{failInsert: true}}
		code := runFakeHandler(t, st, &fakeRecorder{}, "ei", []byte("x"))
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("supersede_fail_version_path", func(t *testing.T) {
		prev := store.Evidence{ID: uuid.New(), Version: 1, FileHashSHA256: "h"}
		st := &fakeEvidenceStore{tx: &fakeEvidenceTx{latest: &prev, failSupersede: true}}
		code := runFakeHandler(t, st, &fakeRecorder{}, "ei", []byte("x"))
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("record_fail", func(t *testing.T) {
		st := &fakeEvidenceStore{tx: &fakeEvidenceTx{}}
		code := runFakeHandler(t, st, &fakeRecorder{failRecord: true}, "ei", []byte("x"))
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("commit_fail", func(t *testing.T) {
		st := &fakeEvidenceStore{tx: &fakeEvidenceTx{failCommit: true}}
		code := runFakeHandler(t, st, &fakeRecorder{}, "ei", []byte("x"))
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("version_event_success", func(t *testing.T) {
		prev := store.Evidence{ID: uuid.New(), Version: 1, FileHashSHA256: "different"}
		st := &fakeEvidenceStore{tx: &fakeEvidenceTx{latest: &prev}}
		code := runFakeHandler(t, st, &fakeRecorder{}, "ei", []byte("v2"))
		assert.Equal(t, http.StatusCreated, code, "버전 이벤트 정상 경로 → 201")
	})
}
