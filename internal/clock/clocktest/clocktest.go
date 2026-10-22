// Package clocktest installs deterministic clocks for tests.
package clocktest

import (
	"sync"
	"testing"

	"github.com/jpl-au/llmd/internal/clock"
)

// Sequence installs a clock that returns start on its first call and
// advances by step milliseconds on each later call, so every write gets
// a distinct, increasing timestamp. The real clock is restored when the
// test ends.
func Sequence(tb testing.TB, start, step int64) {
	tb.Helper()
	var mu sync.Mutex
	next := start
	install(tb, func() int64 {
		mu.Lock()
		defer mu.Unlock()
		v := next
		next += step
		return v
	})
}

// Fixed installs a clock stuck at ms. The real clock is restored when
// the test ends.
func Fixed(tb testing.TB, ms int64) {
	tb.Helper()
	install(tb, func() int64 { return ms })
}

func install(tb testing.TB, fn func() int64) {
	orig := clock.Now
	clock.Now = fn
	tb.Cleanup(func() { clock.Now = orig })
}
