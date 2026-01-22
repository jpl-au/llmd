// Package host provides the Host API exposed to plugins via WASM imports.
package host

// HostAPI provides the functions that plugins can call.
// These are exposed as WASM imports and call into the llmd.Store.
type HostAPI struct {
	host *Host
}

// NewHostAPI creates a new host API instance.
func NewHostAPI(h *Host) *HostAPI {
	return &HostAPI{host: h}
}

// Documents API

// DocumentRead reads a document by path.
func (api *HostAPI) DocumentRead(path string, version *int) ([]byte, error) {
	// TODO: Call h.store.Documents.Read()
	return nil, nil
}

// DocumentWrite writes a document.
func (api *HostAPI) DocumentWrite(path string, content []byte, author, message string) error {
	// TODO: Call h.store.Documents.Write()
	return nil
}

// DocumentList lists documents by prefix.
func (api *HostAPI) DocumentList(prefix string, includeDeleted bool) ([]string, error) {
	// TODO: Call h.store.Documents.List()
	return nil, nil
}

// DocumentDelete soft-deletes a document.
func (api *HostAPI) DocumentDelete(path string, author string) error {
	// TODO: Call h.store.Documents.Delete()
	return nil
}

// Search API

// SearchRegex searches documents with a regex pattern.
func (api *HostAPI) SearchRegex(pattern, path string, ignoreCase bool) ([]byte, error) {
	// TODO: Call h.store.Search.Regex()
	return nil, nil
}

// SearchFullText performs full-text search.
func (api *HostAPI) SearchFullText(query, prefix string) ([]byte, error) {
	// TODO: Call h.store.Search.FullText()
	return nil, nil
}

// SearchGlob finds documents matching a glob pattern.
func (api *HostAPI) SearchGlob(pattern string) ([]string, error) {
	// TODO: Call h.store.Search.Glob()
	return nil, nil
}

// History API

// HistoryList returns version history for a document.
func (api *HostAPI) HistoryList(path string, limit int) ([]byte, error) {
	// TODO: Call h.store.History.List()
	return nil, nil
}

// HistoryDiff compares two versions of a document.
func (api *HostAPI) HistoryDiff(path string, v1, v2 int) ([]byte, error) {
	// TODO: Call h.store.History.Diff()
	return nil, nil
}

// Tags API

// TagAdd adds a tag to a document.
func (api *HostAPI) TagAdd(path, tag, author string) error {
	// TODO: Call h.store.Tags.Add()
	return nil
}

// TagRemove removes a tag from a document.
func (api *HostAPI) TagRemove(path, tag, author string) error {
	// TODO: Call h.store.Tags.Remove()
	return nil
}

// TagList lists tags for a document or all tags.
func (api *HostAPI) TagList(path string) ([]string, error) {
	// TODO: Call h.store.Tags.List()
	return nil, nil
}

// Links API

// LinkAdd creates a link between documents.
func (api *HostAPI) LinkAdd(from, to, tag, author string) error {
	// TODO: Call h.store.Links.Add()
	return nil
}

// LinkRemove removes a link.
func (api *HostAPI) LinkRemove(id, author string) error {
	// TODO: Call h.store.Links.Remove()
	return nil
}

// LinkList lists links for a document.
func (api *HostAPI) LinkList(path string) ([]byte, error) {
	// TODO: Call h.store.Links.List()
	return nil, nil
}

// Entity API (for plugin state)

// EntityRead reads an entity by namespace and path.
func (api *HostAPI) EntityRead(namespace, path string) ([]byte, error) {
	// TODO: Call h.store.Entities.Read()
	return nil, nil
}

// EntityWrite writes an entity.
func (api *HostAPI) EntityWrite(namespace, path string, value []byte, author string) error {
	// TODO: Call h.store.Entities.Write()
	return nil
}

// EntityList lists entities by namespace and prefix.
func (api *HostAPI) EntityList(namespace, prefix string) ([]byte, error) {
	// TODO: Call h.store.Entities.List()
	return nil, nil
}
