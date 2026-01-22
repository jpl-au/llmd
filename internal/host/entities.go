// This file implements entity-related host functions.
//
// Entities are the low-level storage mechanism for structured data in llmd.
// Tags and links are implemented as entities. Most plugins should use the
// higher-level tag and link functions rather than working with entities
// directly.
//
// Entities are organised into namespaces and have a relation (like a path)
// and a value. Each entity has a unique 9-character key.
package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd/core"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	hostpb "github.com/jpl-au/llmd/proto/host"
)

// EntityRead reads an entity by key or relation.
//
// If the Path looks like a 9-character key, reads by key directly. Otherwise,
// searches for an entity matching the relation and returns the first match.
// Returns an error if not found.
func (h *HostFuncs) EntityRead(ctx context.Context, req *hostpb.EntityRequest) (*hostpb.Entity, error) {
	// If path looks like a key (9 chars), read by key
	if len(req.Path) == 9 {
		entity, err := h.store.Entities.Read(ctx, req.Path)
		if err != nil {
			return nil, err
		}

		return &hostpb.Entity{
			Namespace: entity.Namespace,
			Path:      entity.Relation,
			Value:     []byte(entity.Value),
			CreatedAt: entity.CreatedAt,
		}, nil
	}

	// Otherwise list and filter by relation
	list, err := h.store.Entities.List(ctx, req.Namespace, entities.ListOptions{
		Relation: req.Path,
		Limit:    1,
	})
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, entities.ErrNotFound
	}

	entity := list[0]
	return &hostpb.Entity{
		Namespace: entity.Namespace,
		Path:      entity.Relation,
		Value:     []byte(entity.Value),
		CreatedAt: entity.CreatedAt,
	}, nil
}

// EntityWrite creates or updates an entity.
//
// Creates an entity in the specified namespace with the given relation and
// value. A unique key is generated for the entity. Returns the created entity.
func (h *HostFuncs) EntityWrite(ctx context.Context, req *hostpb.EntityWriteRequest) (*hostpb.Entity, error) {
	opts := entities.WriteOptions{
		WriteContext: core.WriteContext{
			Author: req.Author,
			Source: "plugin",
		},
		Relation: req.Path,
	}

	entity, err := h.store.Entities.Write(ctx, req.Namespace, string(req.Value), opts)
	if err != nil {
		return nil, err
	}

	return &hostpb.Entity{
		Namespace: entity.Namespace,
		Path:      entity.Relation,
		Value:     []byte(entity.Value),
		CreatedAt: entity.CreatedAt,
	}, nil
}

// EntityDelete deletes an entity by key or relation.
//
// If the Path looks like a 9-character key, deletes that entity. Otherwise,
// searches for entities matching the relation and deletes all of them.
// Deletion is permanent.
func (h *HostFuncs) EntityDelete(ctx context.Context, req *hostpb.EntityRequest) (*hostpb.Empty, error) {
	opts := entities.DeleteOptions{
		WriteContext: core.WriteContext{
			Author: "plugin",
			Source: "plugin",
		},
	}

	// If path looks like a key, delete by key
	if len(req.Path) == 9 {
		if err := h.store.Entities.Delete(ctx, req.Path, opts); err != nil {
			return nil, err
		}
		return &hostpb.Empty{}, nil
	}

	// Otherwise list and delete all matching
	list, err := h.store.Entities.List(ctx, req.Namespace, entities.ListOptions{
		Relation: req.Path,
	})
	if err != nil {
		return nil, err
	}

	for _, entity := range list {
		if err := h.store.Entities.Delete(ctx, entity.Key, opts); err != nil {
			return nil, err
		}
	}

	return &hostpb.Empty{}, nil
}

// EntityList lists entities in a namespace.
//
// Returns all entities in the namespace, optionally filtered by a relation
// prefix. Results include the entity key, relation, value, and timestamp.
func (h *HostFuncs) EntityList(ctx context.Context, req *hostpb.EntityListRequest) (*hostpb.EntityListResponse, error) {
	opts := entities.ListOptions{
		Relation: req.Prefix,
	}

	list, err := h.store.Entities.List(ctx, req.Namespace, opts)
	if err != nil {
		return nil, err
	}

	result := make([]*hostpb.Entity, len(list))
	for i, entity := range list {
		result[i] = &hostpb.Entity{
			Namespace: entity.Namespace,
			Path:      entity.Relation,
			Value:     []byte(entity.Value),
			CreatedAt: entity.CreatedAt,
		}
	}

	return &hostpb.EntityListResponse{Entities: result}, nil
}
