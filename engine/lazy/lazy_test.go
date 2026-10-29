package lazy

import (
	"errors"
	"sync"
	"testing"
)

// TestOnce is a sequential scenario: a failing setup runs again, and a
// successful one never runs again.
func TestOnce(t *testing.T) {
	var o Once
	calls := 0
	fail := errors.New("not yet")
	f := func() error {
		calls++
		if calls < 3 {
			return fail
		}
		return nil
	}

	for i, want := range []error{fail, fail, nil, nil} {
		if err := o.Do(f); !errors.Is(err, want) {
			t.Fatalf("Do() call %d error = %v, want %v", i+1, err, want)
		}
	}
	if calls != 3 {
		t.Errorf("setup ran %d times, want 3", calls)
	}
}

// TestOnceConcurrent checks that concurrent callers run the setup once
// and all see what it wrote. Run with -race.
func TestOnceConcurrent(t *testing.T) {
	var o Once
	var calls int
	var value string

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			err := o.Do(func() error {
				calls++
				value = "ready"
				return nil
			})
			if err != nil {
				t.Errorf("Do() error = %v", err)
			}
			if value != "ready" {
				t.Errorf("value = %q after Do(), want %q", value, "ready")
			}
		})
	}
	wg.Wait()
	if calls != 1 {
		t.Errorf("setup ran %d times, want 1", calls)
	}
}
