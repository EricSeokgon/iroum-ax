// storage_test.go — T-012/T-017: EvidenceBlobStore 계약 + 데이터 주권 정적 검증
// DC-019 (인터페이스 컴파일/추상화), DC-018 (외부 SDK 미import),
// DC-001 item 3 부분 (stdlib only), SEC-06 (db:// location 비-traversal)
package storage

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvidenceBlobStore_InterfaceCompiles 인터페이스 시그니처 + 추상화 컴파일 검증
// DC-019
func TestEvidenceBlobStore_InterfaceCompiles(t *testing.T) {
	t.Parallel()
	// 컴파일 타임: NewDBBlobStore가 EvidenceBlobStore 인터페이스를 반환
	var bs EvidenceBlobStore = NewDBBlobStore()
	require.NotNil(t, bs)

	id := uuid.New()
	loc, err := bs.Put(context.Background(), id.String(), strings.NewReader("ignored-bytes"))
	require.NoError(t, err)

	// SEC-06 / DC-017: database_blob location은 서버 생성 UUID 기반 'db://evidences/<uuid>'
	re := regexp.MustCompile(`^db://evidences/[0-9a-f-]{36}$`)
	assert.Regexp(t, re, loc, "database_blob location은 db://evidences/<UUID> 형식")
	assert.Equal(t, "db://evidences/"+id.String(), loc)

	// Get은 database_blob PoC에서 미지원 (EvidenceTx 경유)
	_, getErr := bs.Get(context.Background(), loc)
	assert.Error(t, getErr, "database_blob 전략 Get은 명시적 미지원 에러")

	// 빈 key 거부
	_, emptyErr := bs.Put(context.Background(), "", strings.NewReader("x"))
	assert.Error(t, emptyErr)
}

// TestEvidenceStorage_NoExternalSDKImported go list -deps에 외부 SaaS SDK 부재
// DC-018, REQ-EVID-UBI-001
func TestEvidenceStorage_NoExternalSDKImported(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("go", "list", "-deps",
		"github.com/ircp/iroum-ax/apps/control-plane/internal/storage/...",
	).CombinedOutput()
	require.NoError(t, err, "go list -deps 실행 실패: %s", string(out))

	deps := string(out)
	forbidden := []string{
		"github.com/aws/", "aws-sdk",
		"github.com/minio/", "minio-go",
		"cloud.google.com/", "google-cloud-storage",
		"github.com/Azure/", "azure-storage-blob",
	}
	for _, f := range forbidden {
		assert.NotContains(t, deps, f,
			"internal/storage 의존성에 외부 SaaS SDK(%s) 포함 금지 (데이터 주권)", f)
	}
}
