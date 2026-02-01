// This file implements document-related host functions.
//
// Document operations are the core functionality of llmd. These functions allow
// plugins to read, write, edit, delete, restore, list, move, and check existence
// of documents in the store.
//
// All document operations are versioned - writes create new versions rather than
// overwriting, and deleted documents are soft-deleted (can be restored).
package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/debug"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/core"
	hostpb "github.com/jpl-au/llmd/proto/host"
)

// DocumentRead reads a document by path.
//
// If Version is specified in the request, returns that specific version.
// Otherwise returns the latest version. Returns an error if the document
// doesn't exist or the specified version is not found.
func (h *HostFuncs) DocumentRead(ctx context.Context, req *hostpb.ReadRequest) (*hostpb.DocumentResponse, error) {
	debug.Log("DocumentRead", "path", req.Path, "version", req.Version)

	if h.store == nil {
		debug.Log("DocumentRead error", "error", "store not available")
		return &hostpb.DocumentResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	var opts documents.ReadOptions
	if req.Version != nil {
		v := int(*req.Version)
		opts.Version = &v
	}

	doc, err := h.store.Documents.Read(ctx, req.Path, opts)
	if err != nil {
		debug.Log("DocumentRead error", "path", req.Path, "error", err.Error())
		return &hostpb.DocumentResponse{Error: err.Error()}, nil
	}

	debug.Log("DocumentRead success", "path", doc.Path, "version", doc.Version, "size", len(doc.Content))
	return &hostpb.DocumentResponse{
		Success: true,
		Document: &hostpb.Document{
			Path:      doc.Path,
			Content:   []byte(doc.Content),
			Version:   int32(doc.Version),
			Author:    doc.Author,
			Message:   doc.Message,
			CreatedAt: doc.CreatedAt,
			Deleted:   doc.DeletedAt != nil,
		},
	}, nil
}

// DocumentWrite writes a document, creating a new version.
//
// If the document doesn't exist, it is created. If it exists, a new version
// is created. The author and message are recorded in the version history.
func (h *HostFuncs) DocumentWrite(ctx context.Context, req *hostpb.WriteRequest) (*hostpb.DocumentResponse, error) {
	debug.Log("DocumentWrite", "path", req.Path, "size", len(req.Content))

	if h.store == nil {
		debug.Log("DocumentWrite error", "error", "store not available")
		return &hostpb.DocumentResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := documents.WriteOptions{
		Origin: core.Origin{
			Author:  req.Author,
			Source:  "plugin",
			Message: req.Message,
		},
	}

	doc, err := h.store.Documents.Write(ctx, req.Path, string(req.Content), opts)
	if err != nil {
		debug.Log("DocumentWrite error", "path", req.Path, "error", err.Error())
		return &hostpb.DocumentResponse{Error: err.Error()}, nil
	}

	debug.Log("DocumentWrite success", "path", doc.Path, "version", doc.Version)
	return &hostpb.DocumentResponse{
		Success: true,
		Document: &hostpb.Document{
			Path:      doc.Path,
			Content:   []byte(doc.Content),
			Version:   int32(doc.Version),
			Author:    doc.Author,
			Message:   doc.Message,
			CreatedAt: doc.CreatedAt,
		},
	}, nil
}

// DocumentEdit performs a search/replace edit on a document.
//
// This atomically reads the document, replaces all occurrences of OldText with
// NewText, and writes the result as a new version. Returns an error if OldText
// is not found in the document.
func (h *HostFuncs) DocumentEdit(ctx context.Context, req *hostpb.EditRequest) (*hostpb.DocumentResponse, error) {
	if h.store == nil {
		return &hostpb.DocumentResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := documents.EditOptions{
		Origin: core.Origin{
			Author:  req.Author,
			Source:  "plugin",
			Message: req.Message,
		},
	}

	doc, err := h.store.Documents.Edit(ctx, req.Path, req.OldText, req.NewText, opts)
	if err != nil {
		return &hostpb.DocumentResponse{Error: err.Error()}, nil
	}

	return &hostpb.DocumentResponse{
		Success: true,
		Document: &hostpb.Document{
			Path:      doc.Path,
			Content:   []byte(doc.Content),
			Version:   int32(doc.Version),
			Author:    doc.Author,
			Message:   doc.Message,
			CreatedAt: doc.CreatedAt,
		},
	}, nil
}

// DocumentDelete soft-deletes a document.
//
// Soft-deleted documents are marked as deleted but not removed from the store.
// They can be restored using DocumentRestore. The deletion is recorded in the
// version history.
func (h *HostFuncs) DocumentDelete(ctx context.Context, req *hostpb.DeleteRequest) (*hostpb.EmptyResponse, error) {
	if h.store == nil {
		return &hostpb.EmptyResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := documents.DeleteOptions{
		Origin: core.Origin{
			Author: req.Author,
			Source: "plugin",
		},
	}

	if err := h.store.Documents.Delete(ctx, req.Path, opts); err != nil {
		return &hostpb.EmptyResponse{Error: err.Error()}, nil
	}

	return &hostpb.EmptyResponse{Success: true}, nil
}

// DocumentRestore restores a soft-deleted document.
//
// The document is undeleted and becomes visible again. The restoration is
// recorded in the version history. Returns an error if the document doesn't
// exist or is not deleted.
func (h *HostFuncs) DocumentRestore(ctx context.Context, req *hostpb.RestoreRequest) (*hostpb.DocumentResponse, error) {
	if h.store == nil {
		return &hostpb.DocumentResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := documents.RestoreOptions{
		Origin: core.Origin{
			Author: req.Author,
			Source: "plugin",
		},
	}

	if err := h.store.Documents.Restore(ctx, req.Path, opts); err != nil {
		return &hostpb.DocumentResponse{Error: err.Error()}, nil
	}

	doc, err := h.store.Documents.Read(ctx, req.Path)
	if err != nil {
		return &hostpb.DocumentResponse{Error: err.Error()}, nil
	}

	return &hostpb.DocumentResponse{
		Success: true,
		Document: &hostpb.Document{
			Path:      doc.Path,
			Content:   []byte(doc.Content),
			Version:   int32(doc.Version),
			Author:    doc.Author,
			Message:   doc.Message,
			CreatedAt: doc.CreatedAt,
		},
	}, nil
}

// DocumentList lists documents matching a prefix.
//
// Returns document metadata (not content) for all matching documents. If
// IncludeDeleted is true, soft-deleted documents are included. Results are
// ordered by path.
func (h *HostFuncs) DocumentList(ctx context.Context, req *hostpb.ListRequest) (*hostpb.DocumentListResult, error) {
	debug.Log("DocumentList", "prefix", req.Prefix, "includeDeleted", req.IncludeDeleted)

	if h.store == nil {
		debug.Log("DocumentList error", "error", "store not available")
		return &hostpb.DocumentListResult{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := documents.ListOptions{
		Prefix:         req.Prefix,
		IncludeDeleted: req.IncludeDeleted,
	}

	infos, err := h.store.Documents.List(ctx, opts)
	if err != nil {
		debug.Log("DocumentList error", "error", err.Error())
		return &hostpb.DocumentListResult{Error: err.Error()}, nil
	}

	docs := make([]*hostpb.Document, len(infos))
	for i, info := range infos {
		docs[i] = &hostpb.Document{
			Path:      info.Path,
			Version:   int32(info.Version),
			Author:    info.Author,
			Message:   info.Message,
			CreatedAt: info.CreatedAt,
			Deleted:   info.DeletedAt != nil,
		}
	}

	debug.Log("DocumentList success", "count", len(docs))
	return &hostpb.DocumentListResult{Success: true, Documents: docs}, nil
}

// DocumentMove moves a document to a new path.
//
// The document is moved atomically - it's copied to the new path, then the
// old path is deleted. Version history is preserved. Returns an error if
// the source doesn't exist or the destination already exists.
func (h *HostFuncs) DocumentMove(ctx context.Context, req *hostpb.MoveRequest) (*hostpb.DocumentResponse, error) {
	if h.store == nil {
		return &hostpb.DocumentResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := documents.MoveOptions{
		Origin: core.Origin{
			Author: req.Author,
			Source: "plugin",
		},
	}

	if err := h.store.Documents.Move(ctx, req.Source, req.Dest, opts); err != nil {
		return &hostpb.DocumentResponse{Error: err.Error()}, nil
	}

	doc, err := h.store.Documents.Read(ctx, req.Dest)
	if err != nil {
		return &hostpb.DocumentResponse{Error: err.Error()}, nil
	}

	return &hostpb.DocumentResponse{
		Success: true,
		Document: &hostpb.Document{
			Path:      doc.Path,
			Content:   []byte(doc.Content),
			Version:   int32(doc.Version),
			Author:    doc.Author,
			Message:   doc.Message,
			CreatedAt: doc.CreatedAt,
		},
	}, nil
}

// DocumentExists checks if a document exists at the given path.
//
// Returns true if the document exists and is not deleted. This is more
// efficient than calling Read when you only need to check existence.
func (h *HostFuncs) DocumentExists(ctx context.Context, req *hostpb.ExistsRequest) (*hostpb.ExistsResult, error) {
	if h.store == nil {
		return &hostpb.ExistsResult{Error: ErrStoreNotAvailable.Error()}, nil
	}
	exists, err := h.store.Documents.Exists(ctx, req.Path)
	if err != nil {
		return &hostpb.ExistsResult{Error: err.Error()}, nil
	}

	return &hostpb.ExistsResult{Success: true, Exists: exists}, nil
}
