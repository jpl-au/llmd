package settings_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/engine/enginetest"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// recorder keeps the events the bus delivers.
type recorder struct{ events []pkgevents.Event }

func (r *recorder) HandleEvent(_ context.Context, e pkgevents.Event) error {
	r.events = append(r.events, e)
	return nil
}

// TestSettings checks that a key never set has no value, that the
// newest Set is the value, and that each Set emits SettingChanged.
func TestSettings(t *testing.T) {
	s := enginetest.Open(t)
	ctx := t.Context()
	rec := &recorder{}
	s.Bus().Subscribe(rec)

	if value, ok, err := s.Settings.Value(ctx, "agent.claude-code.harness"); err != nil || ok || value != "" {
		t.Fatalf("Value before Set = %q, %v, %v, want \"\", false, nil", value, ok, err)
	}

	for _, value := range []string{"first", "second"} {
		if err := s.Settings.Set(ctx, "agent.claude-code.harness", value, "alice"); err != nil {
			t.Fatalf("Set(%s) error = %v", value, err)
		}
	}
	if value, ok, err := s.Settings.Value(ctx, "agent.claude-code.harness"); err != nil || !ok || value != "second" {
		t.Errorf("Value after two Sets = %q, %v, %v, want \"second\", true, nil", value, ok, err)
	}
	if value, ok, err := s.Settings.Value(ctx, "agent.gemini.harness"); err != nil || ok {
		t.Errorf("Value of another key = %q, %v, %v, want no value", value, ok, err)
	}

	var changed []pkgevents.Event
	for _, e := range rec.events {
		if e.Type == pkgevents.SettingChanged {
			changed = append(changed, e)
		}
	}
	if len(changed) != 2 {
		t.Fatalf("SettingChanged events = %d, want 2", len(changed))
	}
	if e := changed[1]; e.Key != "agent.claude-code.harness" || e.Author != "alice" || e.Metadata["value"] != "second" {
		t.Errorf("last SettingChanged = %+v, want key agent.claude-code.harness, author alice, value second", e)
	}
}
