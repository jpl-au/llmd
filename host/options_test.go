package host

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine/enginetest"
	"github.com/jpl-au/llmd/sdk"
	"github.com/jpl-au/llmd/validate"
)

// stubGit is a GitStore that reports git as unavailable.
type stubGit struct{ sdk.GitStore }

func (stubGit) Available() error { return errors.New("no git in this test") }

// TestOptions checks that WithGit and WithLimits replace the defaults
// and reach the command context, and that a host built for a single
// call keeps them.
func TestOptions(t *testing.T) {
	store := enginetest.Open(t)
	lim := validate.Limits{MaxPathLen: 8, MaxContentLen: 1 << 20}
	h := setup(store, WithGit(stubGit{}), WithLimits(lim))
	c := h.Context(t.Context(), "alice")

	if err := c.Git.Available(); err == nil || err.Error() != "no git in this test" {
		t.Errorf("Git.Available() = %v, want the stub's error", err)
	}

	short, long := "ok", "far/too/long/a/path"
	if err := c.Documents.Write(short, []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Errorf("Write(%q) under a limit of %d error = %v", short, lim.MaxPathLen, err)
	}
	if err := c.Documents.Write(long, []byte("x"), sdk.WriteOpts{Author: "alice"}); !errors.Is(err, sdk.ErrInvalidArg) {
		t.Errorf("Write(%q) under a limit of %d error = %v, want sdk.ErrInvalidArg", long, lim.MaxPathLen, err)
	}

	again := setup(nil, h.opts...)
	if again.lim != lim {
		t.Errorf("options not carried to a new host: limits = %+v, want %+v", again.lim, lim)
	}
}
