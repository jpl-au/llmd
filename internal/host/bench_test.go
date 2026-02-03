package host

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/core"
)

func BenchmarkNewHost(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		h, err := New(ctx, nil)
		if err != nil {
			b.Fatal(err)
		}
		h.Close(ctx)
	}
}

func BenchmarkLoadEmbedded(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		h, err := New(ctx, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := h.load(ctx, PluginEmbed, "core"); err != nil {
			b.Fatal(err)
		}
		h.Close(ctx)
	}
}

func BenchmarkLoadPlugins(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		h, err := New(ctx, nil)
		if err != nil {
			b.Fatal(err)
		}
		if err := h.LoadPlugins(ctx); err != nil {
			b.Fatal(err)
		}
		h.Close(ctx)
	}
}

func BenchmarkExecuteCommand(b *testing.B) {
	ctx := context.Background()
	store, err := llmd.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}

	// Write a test document
	_, err = store.Documents.Write(ctx, "test/doc", "hello world", documents.WriteOptions{
		Origin: core.Origin{Author: "test", Source: "cli"},
	})
	if err != nil {
		b.Fatal(err)
	}

	h, err := New(ctx, store)
	if err != nil {
		b.Fatal(err)
	}

	if err := h.LoadPlugins(ctx); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := h.ExecuteCommand(ctx, "cat", []string{"test/doc"}, nil, nil, "author")
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	h.Close(ctx)
	store.Close()
}

func BenchmarkExecuteCommandLS(b *testing.B) {
	ctx := context.Background()
	store, err := llmd.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}

	// Write some test documents
	for i := range 10 {
		_, err = store.Documents.Write(ctx, "test/doc"+string(rune('a'+i)), "content", documents.WriteOptions{
			Origin: core.Origin{Author: "test", Source: "cli"},
		})
		if err != nil {
			b.Fatal(err)
		}
	}

	h, err := New(ctx, store)
	if err != nil {
		b.Fatal(err)
	}

	if err := h.LoadPlugins(ctx); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := h.ExecuteCommand(ctx, "ls", []string{"test/"}, nil, nil, "author")
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	h.Close(ctx)
	store.Close()
}
