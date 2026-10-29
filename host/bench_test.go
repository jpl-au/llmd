package host

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/engine/enginetest"
)

func BenchmarkNewHost(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New()
	}
}

func BenchmarkNewHostWithStore(b *testing.B) {
	store := enginetest.Open(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setup(store)
	}
}

func BenchmarkExecLS(b *testing.B) {
	store := enginetest.Open(b)

	h := setup(store)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Exec(context.Background(), "ls", nil, "", nil, "")
	}
}

func BenchmarkExecCat(b *testing.B) {
	store := enginetest.Open(b)

	h := setup(store)

	// Create a test document
	_, _ = h.Exec(context.Background(), "write", []string{"test.md"}, "bench", []byte("# Test"), "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Exec(context.Background(), "cat", []string{"test.md"}, "", nil, "")
	}
}

func BenchmarkExecWrite(b *testing.B) {
	store := enginetest.Open(b)

	h := setup(store)
	content := []byte("# Benchmark Test\n\nThis is test content.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Exec(context.Background(), "write", []string{"bench.md"}, "bench", content, "")
	}
}
