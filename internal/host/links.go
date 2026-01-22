// This file implements link-related host functions.
//
// Links create relationships between documents. Unlike tags (which categorise
// a single document), links connect two documents with an optional label
// describing the relationship. Links are bidirectional - you can query either
// direction.
package host

import (
	"context"
	"strings"

	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/internal/llmd/links"
	hostpb "github.com/jpl-au/llmd/proto/host"
)

// LinkAdd creates a link between two documents.
//
// Creates a directed link from the From document to the To document, with
// an optional Tag describing the relationship. If the link already exists,
// this is a no-op. Returns the created or existing link.
func (h *HostFuncs) LinkAdd(ctx context.Context, req *hostpb.LinkRequest) (*hostpb.Link, error) {
	opts := links.Options{
		Origin: core.Origin{
			Author: req.Author,
			Source: "plugin",
		},
		Label: req.Tag,
	}

	link, err := h.store.Links.Add(ctx, req.From, req.To, opts)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, err
	}

	if link == nil {
		return &hostpb.Link{
			From: req.From,
			To:   req.To,
			Tag:  req.Tag,
		}, nil
	}

	return &hostpb.Link{
		Id:   link.Key,
		From: link.Relation,
		To:   link.Value.To,
		Tag:  link.Value.Label,
	}, nil
}

// LinkRemove removes a link by its ID.
//
// The link is permanently deleted. The ID should be obtained from a previous
// LinkAdd or LinkList call. Returns an error if the link doesn't exist.
func (h *HostFuncs) LinkRemove(ctx context.Context, req *hostpb.UnlinkRequest) (*hostpb.Empty, error) {
	opts := entities.DeleteOptions{
		Origin: core.Origin{
			Author: req.Author,
			Source: "plugin",
		},
	}

	if err := h.store.Entities.Delete(ctx, req.Id, opts); err != nil {
		return nil, err
	}

	return &hostpb.Empty{}, nil
}

// LinkList lists all links for a document.
//
// Returns both incoming and outgoing links. Each link includes its ID, the
// source and target paths, and the optional tag. If the document has no
// links, returns an empty list.
func (h *HostFuncs) LinkList(ctx context.Context, req *hostpb.LinkListRequest) (*hostpb.LinkListResponse, error) {
	opts := links.Options{
		Direction: links.Both,
	}

	linkList, err := h.store.Links.List(ctx, req.Path, opts)
	if err != nil {
		return nil, err
	}

	result := make([]*hostpb.Link, len(linkList))
	for i, l := range linkList {
		result[i] = &hostpb.Link{
			Id:   l.Key,
			From: l.Relation,
			To:   l.Value.To,
			Tag:  l.Value.Label,
		}
	}

	return &hostpb.LinkListResponse{Links: result}, nil
}
