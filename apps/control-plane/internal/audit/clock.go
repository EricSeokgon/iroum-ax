// clock.go — 시각 주입 추상화 (SPEC-AX-EVID-001 R-EVID-007 / T-018 REFACTOR)
// recorder.go의 직접 time.Now().UTC() 호출은 테스트 비친화적이므로,
// Clock 인터페이스로 추상화하여 테스트에서 고정 시각을 주입할 수 있게 한다.
// 기본 구현(systemClock)은 기존 동작(time.Now().UTC())과 byte-identical — 동작 변경 없음.
package audit

import "time"

// Clock 현재 UTC 시각을 제공하는 추상화 — 테스트에서 고정 시각 주입 가능
type Clock interface {
	// NowUTC 현재 시각을 UTC로 반환
	NowUTC() time.Time
}

// systemClock 실제 시스템 시각을 반환하는 기본 Clock 구현
// 기존 time.Now().UTC() 호출과 동일 동작 (REFACTOR — behavior 불변)
type systemClock struct{}

// NowUTC time.Now().UTC()를 반환 (기존 recorder.go 동작과 byte-identical)
func (systemClock) NowUTC() time.Time { return time.Now().UTC() }

// defaultClock Recorder가 명시적 Clock 미주입 시 사용하는 기본값
var defaultClock Clock = systemClock{}
