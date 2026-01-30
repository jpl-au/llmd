// This file implements tag-related host functions.
//
// Tags provide a way to categorise and organise documents. Each document can
// have multiple tags, and tags can be used to filter and search documents.
// Tags are stored as entities associated with the document path.
package host

import (
	"context"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd/tags"
	"github.com/jpl-au/llmd/pkg/model/core"
	hostpb "github.com/jpl-au/llmd/proto/host"
)

// TagAdd adds a tag to a document.
//
// If the tag already exists on the document, this is a no-op. The addition
// is recorded in the version history with the specified author.
func (h *HostFuncs) TagAdd(ctx context.Context, req *hostpb.TagRequest) (*hostpb.Empty, error) {
	opts := tags.Options{
		Origin: core.Origin{
			Author: req.Author,
			Source: "plugin",
		},
	}

	_, err := h.store.Tags.Add(ctx, req.Path, req.Tag, opts)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, err
	}

	return &hostpb.Empty{}, nil
}

// TagRemove removes a tag from a document.
//
// If the tag doesn't exist on the document, returns an error. The removal
// is recorded in the version history with the specified author.
func (h *HostFuncs) TagRemove(ctx context.Context, req *hostpb.TagRequest) (*hostpb.Empty, error) {
	opts := tags.Options{
		Origin: core.Origin{
			Author: req.Author,
			Source: "plugin",
		},
	}

	if err := h.store.Tags.Remove(ctx, req.Path, req.Tag, opts); err != nil {
		return nil, err
	}

	return &hostpb.Empty{}, nil
}

// TagList lists all tags on a document.
//
// Returns the tag names in alphabetical order. If the document doesn't exist
// or has no tags, returns an empty list.
func (h *HostFuncs) TagList(ctx context.Context, req *hostpb.TagListRequest) (*hostpb.TagListResponse, error) {
	tagList, err := h.store.Tags.List(ctx, req.Path)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(tagList))
	for i, t := range tagList {
		names[i] = t.Value.Tag
	}

	return &hostpb.TagListResponse{Tags: names}, nil
}
