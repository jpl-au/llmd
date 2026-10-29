// Package lazy runs one-time setup that may fail and be retried.
//
// sync.Once keeps the first result, so a setup that fails once (work.db
// briefly locked, a full disk) stays failed for the life of the
// process, which for the server means until it is restarted. Once
// records only success: a failed setup runs again on the next call.
package lazy

import (
	"sync"
	"sync/atomic"
)

// Once runs a setup function until it succeeds once. The zero value is
// ready to use. Safe for concurrent use; calls wait while another call
// is running the setup.
type Once struct {
	done atomic.Bool
	mu   sync.Mutex
}

// Do runs f unless an earlier call to f succeeded, and returns f's
// error. Writes f makes before returning nil are visible to every
// caller that Do returns nil to.
func (o *Once) Do(f func() error) error {
	if o.done.Load() {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.done.Load() {
		return nil
	}
	if err := f(); err != nil {
		return err
	}
	o.done.Store(true)
	return nil
}
