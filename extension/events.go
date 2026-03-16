// events.go defines the event types for extension notifications.
//
// Separated from extension.go to isolate the event system. Events enable
// extensions to react to document changes without modifying core logic.
//
// Design: Events are fire-and-forget notifications, not approval requests.
// Extensions cannot block or veto operations via events - they observe
// after the fact. This keeps the core system simple and predictable.
// If approval workflows are needed, a separate hook system should be added.

package extension

// EventType identifies the kind of event.
type EventType string

const (
	EventDocumentWrite   EventType = "document:write"
	EventDocumentDelete  EventType = "document:delete"
	EventDocumentRestore EventType = "document:restore"
	EventDocumentMove    EventType = "document:move"
	EventTagAdd          EventType = "tag:add"
	EventTagRemove       EventType = "tag:remove"
	EventLinkCreate      EventType = "link:create"
	EventLinkRemove      EventType = "link:remove"
	EventAuditCreate     EventType = "audit:create"
	EventAuditReply      EventType = "audit:reply"
	EventAuditResolve    EventType = "audit:resolve"
	EventAuditDelete     EventType = "audit:delete"
	EventAuditRestore    EventType = "audit:restore"
	EventTaskCreate      EventType = "task:create"
	EventTaskMove        EventType = "task:move"
	EventTaskUpdate      EventType = "task:update"
	EventTaskDelete      EventType = "task:delete"
	EventTaskRestore     EventType = "task:restore"
)

// Event is the base interface for all events.
type Event interface {
	EventType() EventType
	EventPath() string
}

// DocumentWriteEvent is fired after a document write.
type DocumentWriteEvent struct {
	Path    string
	Version int
	Author  string
	Message string
	Content string
}

func (e DocumentWriteEvent) EventType() EventType { return EventDocumentWrite }
func (e DocumentWriteEvent) EventPath() string    { return e.Path }

// DocumentDeleteEvent is fired after a document is soft-deleted.
// Version is 0 when all versions are deleted, or the specific version number
// when only one version was deleted.
type DocumentDeleteEvent struct {
	Path    string
	Version int
}

func (e DocumentDeleteEvent) EventType() EventType { return EventDocumentDelete }
func (e DocumentDeleteEvent) EventPath() string    { return e.Path }

// DocumentRestoreEvent is fired after a document is restored.
type DocumentRestoreEvent struct {
	Path    string
	Version int
}

func (e DocumentRestoreEvent) EventType() EventType { return EventDocumentRestore }
func (e DocumentRestoreEvent) EventPath() string    { return e.Path }

// DocumentMoveEvent is fired after a document is moved to a new path.
type DocumentMoveEvent struct {
	Path    string
	OldPath string
	Version int
}

func (e DocumentMoveEvent) EventType() EventType { return EventDocumentMove }
func (e DocumentMoveEvent) EventPath() string    { return e.Path }

// TagEvent is fired after a tag is added or removed.
type TagEvent struct {
	Path   string
	Tag    string
	Source string
	Added  bool // true=added, false=removed
}

func (e TagEvent) EventType() EventType {
	if e.Added {
		return EventTagAdd
	}
	return EventTagRemove
}
func (e TagEvent) EventPath() string { return e.Path }

// LinkEvent is fired after a link is created or removed.
type LinkEvent struct {
	ID       string
	FromPath string
	ToPath   string
	Tag      string
	Created  bool // true=created, false=removed
}

func (e LinkEvent) EventType() EventType {
	if e.Created {
		return EventLinkCreate
	}
	return EventLinkRemove
}
func (e LinkEvent) EventPath() string { return e.FromPath }

// AuditEvent is fired after an audit mutation (create, reply, resolve,
// delete, restore).
type AuditEvent struct {
	Type     EventType
	ID       string
	Target   string
	Author   string
	Status   string
	ParentID string
}

func (e AuditEvent) EventType() EventType { return e.Type }
func (e AuditEvent) EventPath() string    { return e.Target }

// TaskEvent is fired after a task mutation (create, move, update,
// delete, restore).
type TaskEvent struct {
	Type   EventType
	Key    string
	Path   string
	Author string
	Title  string
	Status string
}

func (e TaskEvent) EventType() EventType { return e.Type }
func (e TaskEvent) EventPath() string    { return e.Path }

// EventHandler is implemented by extensions that want to receive events.
type EventHandler interface {
	HandleEvent(ctx Context, e Event) error
}
