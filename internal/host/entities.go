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

	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/pkg/model/core"
	hostpb "github.com/jpl-au/llmd/proto/host"
)

// EntityRead reads an entity by key or relation.
//
// If the Path looks like a 9-character key, reads by key directly. Otherwise,
// searches for an entity matching the relation and returns the first match.
// Returns an error if not found.
func (h *HostFuncs) EntityRead(ctx context.Context, req *hostpb.EntityRequest) (*hostpb.EntityResponse, error) {
	if h.store == nil {
		return &hostpb.EntityResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	// If path looks like a key (9 chars), read by key
	if len(req.Path) == 9 {
		entity, err := h.store.Entities.Read(ctx, req.Path)
		if err != nil {
			return &hostpb.EntityResponse{Error: err.Error()}, nil
		}

		return &hostpb.EntityResponse{
			Success: true,
			Entity: &hostpb.Entity{
				Namespace: entity.Namespace,
				Path:      entity.Relation,
				Value:     []byte(entity.Value),
				CreatedAt: entity.CreatedAt,
			},
		}, nil
	}

	// Otherwise list and filter by relation
	list, err := h.store.Entities.List(ctx, req.Namespace, entities.ListOptions{
		Relation: req.Path,
		Limit:    1,
	})
	if err != nil {
		return &hostpb.EntityResponse{Error: err.Error()}, nil
	}

	if len(list) == 0 {
		return &hostpb.EntityResponse{Error: entities.ErrNotFound.Error()}, nil
	}

	entity := list[0]
	return &hostpb.EntityResponse{
		Success: true,
		Entity: &hostpb.Entity{
			Namespace: entity.Namespace,
			Path:      entity.Relation,
			Value:     []byte(entity.Value),
			CreatedAt: entity.CreatedAt,
		},
	}, nil
}

// EntityWrite creates or updates an entity.
//
// Creates an entity in the specified namespace with the given relation and
// value. A unique key is generated for the entity. Returns the created entity.
func (h *HostFuncs) EntityWrite(ctx context.Context, req *hostpb.EntityWriteRequest) (*hostpb.EntityResponse, error) {
	if h.store == nil {
		return &hostpb.EntityResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := entities.WriteOptions{
		Origin: core.Origin{
			Author: req.Author,
			Source: "plugin",
		},
		Relation: req.Path,
	}

	entity, err := h.store.Entities.Write(ctx, req.Namespace, string(req.Value), opts)
	if err != nil {
		return &hostpb.EntityResponse{Error: err.Error()}, nil
	}

	return &hostpb.EntityResponse{
		Success: true,
		Entity: &hostpb.Entity{
			Namespace: entity.Namespace,
			Path:      entity.Relation,
			Value:     []byte(entity.Value),
			CreatedAt: entity.CreatedAt,
		},
	}, nil
}

// EntityDelete deletes an entity by key or relation.
//
// If the Path looks like a 9-character key, deletes that entity. Otherwise,
// searches for entities matching the relation and deletes all of them.
// Deletion is permanent.
func (h *HostFuncs) EntityDelete(ctx context.Context, req *hostpb.EntityRequest) (*hostpb.EmptyResponse, error) {
	if h.store == nil {
		return &hostpb.EmptyResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := entities.DeleteOptions{
		Origin: core.Origin{
			Author: "plugin",
			Source: "plugin",
		},
	}

	// If path looks like a key, delete by key
	if len(req.Path) == 9 {
		if err := h.store.Entities.Delete(ctx, req.Path, opts); err != nil {
			return &hostpb.EmptyResponse{Error: err.Error()}, nil
		}
		return &hostpb.EmptyResponse{Success: true}, nil
	}

	// Otherwise list and delete all matching
	list, err := h.store.Entities.List(ctx, req.Namespace, entities.ListOptions{
		Relation: req.Path,
	})
	if err != nil {
		return &hostpb.EmptyResponse{Error: err.Error()}, nil
	}

	for _, entity := range list {
		if err := h.store.Entities.Delete(ctx, entity.Key, opts); err != nil {
			return &hostpb.EmptyResponse{Error: err.Error()}, nil
		}
	}

	return &hostpb.EmptyResponse{Success: true}, nil
}

// EntityList lists entities in a namespace.
//
// Returns all entities in the namespace, optionally filtered by a relation
// prefix. Results include the entity key, relation, value, and timestamp.
func (h *HostFuncs) EntityList(ctx context.Context, req *hostpb.EntityListRequest) (*hostpb.EntityListResult, error) {
	if h.store == nil {
		return &hostpb.EntityListResult{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := entities.ListOptions{
		Relation: req.Prefix,
	}

	list, err := h.store.Entities.List(ctx, req.Namespace, opts)
	if err != nil {
		return &hostpb.EntityListResult{Error: err.Error()}, nil
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

	return &hostpb.EntityListResult{Success: true, Entities: result}, nil
}
