package host

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
)

func BenchmarkNewHost(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New()
	}
}

func BenchmarkNewHostWithStore(b *testing.B) {
	store, err := llmd.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := store.Close(); err != nil {
			b.Errorf("closing store: %v", err)
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setup(store)
	}
}

func BenchmarkExecLS(b *testing.B) {
	store, err := llmd.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := store.Close(); err != nil {
			b.Errorf("closing store: %v", err)
		}
	})

	h := setup(store)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Exec(context.Background(), "ls", nil, "", nil, "")
	}
}

func BenchmarkExecCat(b *testing.B) {
	store, err := llmd.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := store.Close(); err != nil {
			b.Errorf("closing store: %v", err)
		}
	})

	h := setup(store)

	// Create a test document
	_, _ = h.Exec(context.Background(), "write", []string{"test.md"}, "bench", []byte("# Test"), "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Exec(context.Background(), "cat", []string{"test.md"}, "", nil, "")
	}
}

func BenchmarkExecWrite(b *testing.B) {
	store, err := llmd.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := store.Close(); err != nil {
			b.Errorf("closing store: %v", err)
		}
	})

	h := setup(store)
	content := []byte("# Benchmark Test\n\nThis is test content.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Exec(context.Background(), "write", []string{"bench.md"}, "bench", content, "")
	}
}
