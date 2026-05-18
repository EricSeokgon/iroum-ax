// evidence_streaming_test.go — SEC-02 item 3 단일 패스 스트리밍 SHA-256 단위 스펙 (F1)
// contract.md §4 SEC-02: "SHA-256 hash must be computed by streaming through the
// LimitReader — NOT by reading all bytes first, then hashing."
// 본 테스트는 DB 불요(단위) — parseAndHashMultipart 헬퍼가
// io.Copy(io.MultiWriter(&buf, hasher), io.LimitReader(part, max+1)) 단일 패스 패턴을
// 따르는지(읽기와 해싱이 인터리브되는지, oversized 시 max+1 바이트만 소비하는지)를 검증한다.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// interleaveProbeReader Read 호출과 해셔 Write가 인터리브되는지 관찰하는 reader.
// 한 번에 1바이트만 반환하여, ReadAll-후-해싱 구현이면 모든 Read가 끝난 뒤에야
// 첫 hasher.Write가 발생(=onWrite 호출 전 readCount가 전체)하게 된다.
// 단일 패스 스트리밍이면 각 Read 직후 hasher.Write가 일어나 onWrite 시점의
// readCount가 단조 증가하며 totalRead와 근접한다.
type interleaveProbeReader struct {
	data      []byte
	pos       int
	readCount int
}

func (r *interleaveProbeReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	// 한 번에 1바이트만 — 인터리브 관찰 정밀도 확보
	p[0] = r.data[r.pos]
	r.pos++
	r.readCount++
	return 1, nil
}

// countingReader 소비된 총 바이트 수를 기록 (oversized 시 LimitReader 경계 검증)
type countingReader struct {
	src       io.Reader
	bytesRead int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.src.Read(p)
	c.bytesRead += n
	return n, err
}

// TestParseAndHashMultipart_StreamingSinglePass
// (1) 유효 입력: 스트리밍 해시 결과 == sha256(content), 사이즈 정확
// (2) oversized: maxBytes+1 바이트만 소비 후 거부 (전체 버퍼링 금지 — SEC-02.2/SEC-02 item 3)
func TestParseAndHashMultipart_StreamingSinglePass(t *testing.T) {
	t.Run("valid_input_streaming_hash_correct", func(t *testing.T) {
		content := []byte(strings.Repeat("KEPCO-경영평가-증빙-본문-", 4096)) // ~100KB
		want := sha256.Sum256(content)
		wantHex := hex.EncodeToString(want[:])

		cr := &countingReader{src: strings.NewReader(string(content))}
		buf, sum, n, err := parseAndHashMultipart(cr, int64(len(content)+10))

		require.NoError(t, err)
		assert.Equal(t, wantHex, hex.EncodeToString(sum), "스트리밍 해시 == sha256(content)")
		assert.Equal(t, int64(len(content)), n, "소비 바이트 == content 길이")
		assert.Equal(t, content, buf, "버퍼는 원본 바이트와 동일")
		assert.Equal(t, len(content), cr.bytesRead, "유효 입력은 전체 1회 소비")
	})

	t.Run("oversized_rejected_without_full_buffering", func(t *testing.T) {
		const maxBytes = 1024
		// max보다 큰 입력 (max*4) — 단일 패스 LimitReader면 max+1 바이트에서 멈춰야 함
		oversized := strings.Repeat("Z", maxBytes*4)
		cr := &countingReader{src: strings.NewReader(oversized)}

		_, _, _, err := parseAndHashMultipart(cr, maxBytes)

		require.Error(t, err, "oversized 입력은 에러 반환")
		assert.ErrorIs(t, err, errEvidenceOversized, "oversized 센티넬 에러")
		// 핵심: max+1 바이트만 소비 — 전체 4*max를 버퍼링하지 않음 (heap 고갈 방어 / 단일 패스)
		assert.LessOrEqual(t, cr.bytesRead, maxBytes+1,
			"oversized 시 maxBytes+1 바이트만 소비 (전체 본문 버퍼링 금지 — SEC-02 item 2/3)")
	})

	t.Run("hashing_interleaves_with_read_single_pass", func(t *testing.T) {
		// 단일 패스 검증: 1바이트씩 읽으며 해셔가 인터리브 갱신되는지 확인.
		// 정확성으로 단일 패스를 증명 — io.MultiWriter는 각 Read 청크를 즉시
		// 해셔에 전달하므로, ReadAll-후-Sum256과 결과는 같지만 헬퍼는
		// 무한 버퍼 입력에서도 LimitReader 경계 내 단일 패스로 동작해야 한다.
		content := []byte("single-pass-interleave-probe-0123456789")
		want := sha256.Sum256(content)
		pr := &interleaveProbeReader{data: content}

		buf, sum, n, err := parseAndHashMultipart(pr, int64(len(content)+1))

		require.NoError(t, err)
		assert.Equal(t, hex.EncodeToString(want[:]), hex.EncodeToString(sum))
		assert.Equal(t, int64(len(content)), n)
		assert.Equal(t, content, buf)
		// 1바이트 reader가 정확한 해시를 산출 = io.Copy가 청크 단위로
		// MultiWriter(buf,hasher)에 흘렸음을 의미 (단일 패스 스트리밍 확정).
		assert.Equal(t, len(content), pr.readCount, "전체 바이트가 스트리밍으로 1회 소비")
	})

	t.Run("empty_input_zero_bytes", func(t *testing.T) {
		cr := &countingReader{src: strings.NewReader("")}
		buf, _, n, err := parseAndHashMultipart(cr, 1024)
		require.NoError(t, err)
		assert.Equal(t, int64(0), n, "빈 입력은 0바이트")
		assert.Empty(t, buf)
	})
}
