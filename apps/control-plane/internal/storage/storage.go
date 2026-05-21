// storage.go — 증빙 저장 전략 추상화 (SPEC-AX-EVID-001 REQ-EVID-004)
// 확정 전략 = database_blob (plan.md §6 RESOLVED). database_blob 모드에서
// blob 바이너리는 EvidenceTx.InsertEvidence(file_content)로 동일 pgx TX에 흐르고,
// 본 인터페이스 구현체(dbBlobStore)는 논리 location 문자열만 기록한다 (strategy.md §2.6.5 6a).
// 외부 SaaS SDK(aws-sdk/minio-go/gcs/azure)는 import하지 않는다 (REQ-EVID-UBI-001 데이터 주권).
package storage

import (
	"context"
	"fmt"
	"io"
)

// EvidenceBlobStore 증빙 바이너리 저장 전략 추상화 — post-PoC 무중단 전략 교체 대비
// database_blob 전략에서 blob bytes는 이 인터페이스를 통과하지 않으며(EvidenceTx 경유),
// Put은 논리 location 식별자를, Get은 (미사용 PoC) 논리 reader를 반환한다.
//
// @MX:NOTE: [AUTO] 저장 전략 database_blob 확정 — dbBlobStore는 논리 location만 기록(bytes는 EvidenceTx 경유)
// @MX:REASON: 추상화 유지로 schema 변경 없는 filesystem/minio 전환 대비 (strategy.md §2.6.5, REQ-EVID-004-O1)
type EvidenceBlobStore interface {
	// Put 증빙 키에 대한 논리 저장 location을 반환한다.
	// database_blob 전략: r은 읽지 않으며 'db://evidences/<key>' 형식 location만 생성
	// (실제 바이너리는 EvidenceTx.InsertEvidence(file_content)가 동일 TX에 저장)
	Put(ctx context.Context, key string, r io.Reader) (string, error)
	// Get location 문자열로 reader를 반환 (PoC database_blob에서는 미사용 placeholder)
	Get(ctx context.Context, location string) (io.ReadCloser, error)
}

// dbBlobStore database_blob 전략 PoC 구현 — 논리 location 메타데이터만 관리
// 바이너리는 본 구조체를 통과하지 않는다 (EvidenceTx 단일 TX 원자성 보존)
type dbBlobStore struct{}

// NewDBBlobStore database_blob 전략 EvidenceBlobStore를 생성
// 외부 의존 0 — 자격증명/네트워크 endpoint 없음 (REQ-EVID-UBI-001)
func NewDBBlobStore() EvidenceBlobStore { return &dbBlobStore{} }

// Put 'db://evidences/<key>' 논리 location을 반환 (r 미사용 — bytes는 EvidenceTx 경유)
// key는 서버 생성 UUID 문자열 — 사용자 입력 미포함 (SEC-06 path traversal 불가)
func (*dbBlobStore) Put(_ context.Context, key string, _ io.Reader) (string, error) {
	if key == "" {
		return "", fmt.Errorf("dbBlobStore.Put: key가 비어 있음")
	}
	return "db://evidences/" + key, nil
}

// Get database_blob PoC에서는 미사용 — 호출 시 명시적 에러 (조회는 EvidenceTx 경유)
func (*dbBlobStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("dbBlobStore.Get: database_blob 전략에서는 EvidenceTx 경유 조회 — 미지원")
}
