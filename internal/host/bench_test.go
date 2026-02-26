package host

import (
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
	defer store.Close()

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
	defer store.Close()

	h := setup(store)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Exec("ls", nil, "", nil, "")
	}
}

func BenchmarkExecCat(b *testing.B) {
	store, err := llmd.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	h := setup(store)

	// Create a test document
	_, _ = h.Exec("write", []string{"test.md"}, "bench", []byte("# Test"), "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Exec("cat", []string{"test.md"}, "", nil, "")
	}
}

func BenchmarkExecWrite(b *testing.B) {
	store, err := llmd.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	h := setup(store)
	content := []byte("# Benchmark Test\n\nThis is test content.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Exec("write", []string{"bench.md"}, "bench", content, "")
	}
}
