// refresh_store_ext_test.go — fakeRefreshStore 확장 메서드
// FamilyFinder 인터페이스 구현: jti → familyID 역방향 조회
// Sprint 6 GREEN: RefreshService stub 모드에서 family invalidation 지원
package auth_test

import "context"

// FindFamilyByJTI — jti가 속한 familyID를 역방향 조회한다
// families 맵을 순회하여 jti를 포함하는 familyID를 반환한다.
// 없으면 ("", nil)을 반환한다.
func (f *fakeRefreshStore) FindFamilyByJTI(_ context.Context, jti string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for familyID, members := range f.families {
		if _, ok := members[jti]; ok {
			return familyID, nil
		}
	}
	return "", nil
}
