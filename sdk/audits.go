package sdk

import "time"

// AuditStore is the audit management interface for agent-to-agent and
// human-to-agent review threads.
//
// Audits are immutable, insert-only records forming threaded conversations
// attached to documents or tasks. Thread status is derived from the latest
// entry - no record is ever updated. Replies, resolves, and status changes
// are all new records.
//
// Each audit has an [Audit.ID] (9-char base36 key), making it addressable
// across the system and linkable via the standard link interface.
type AuditStore interface {
	// Add creates a top-level audit on a document or task.
	Add(opts AuditOpts) (*Audit, error)

	// Reply adds a response to an existing audit thread. If the parent
	// is itself a reply, the store resolves to the top-level ancestor.
	// Creates a new immutable record.
	Reply(id string, opts AuditOpts) (*Audit, error)

	// Read returns a single audit by ID.
	Read(id string) (*Audit, error)

	// List returns audits matching the filter criteria. Status filters
	// apply to the thread's effective status (latest entry).
	List(opts AuditListOpts) ([]Audit, error)

	// Thread returns a top-level audit and all its replies in
	// chronological order.
	Thread(id string) ([]Audit, error)

	// Resolve inserts a new entry with status "approved" and empty
	// content. Equivalent to Reply with status approved.
	Resolve(id, author string) (*Audit, error)

	// Delete soft-deletes an audit (sets deleted_at).
	Delete(id, author string) error

	// Restore undeletes a soft-deleted audit (clears deleted_at).
	Restore(id, author string) (*Audit, error)

	// Status returns pending audits requiring the given author's
	// attention - threads where the effective status is pending or
	// needs-work and the last entry is not from the author.
	Status(author string, opts AuditStatusOpts) (*AuditStatus, error)
}

// Audit represents a single audit entry - either a top-level review
// or a reply within a thread. Records are immutable once created.
type Audit struct {
	// ID is the unique identifier (9-char base36).
	ID string

	// Target is the document path or task key being audited.
	Target string

	// TargetType is "document" or "task", inferred by the store.
	TargetType string

	// Version is the document version reviewed. Zero for task targets
	// or when no version was pinned.
	Version int

	// Author identifies who created this entry.
	Author string

	// Assignee identifies who needs to act on this audit. Propagates
	// through replies - the effective assignee is from the latest entry.
	Assignee string

	// Status at the time of this entry (e.g. "pending", "approved",
	// "needs-work"). The thread's effective status is the status of
	// its most recent entry.
	Status string

	// Content is the review text. Empty for resolve entries.
	Content string

	// ParentID is the top-level audit ID for replies, or empty for
	// top-level entries.
	ParentID string

	// CreatedAt is the Unix timestamp (milliseconds) when this entry
	// was created.
	CreatedAt int64
}

// AuditOpts configures an audit add or reply operation. All fields
// except Author are optional for Reply (Target is inherited from the
// parent thread).
type AuditOpts struct {
	// Target is the document path or task key. Required for Add,
	// ignored for Reply (inherited from parent).
	Target string

	// Content is the review text. May be empty for resolve-style
	// entries.
	Content string

	// Author identifies who is creating this entry. Required.
	Author string

	// Assignee identifies who should act on this audit. For Reply,
	// inherits from the parent if empty.
	Assignee string

	// Status sets the status on this entry. Default: "pending".
	Status string

	// Version pins to a specific document version. Zero means the
	// store captures the current version at time of write. Ignored
	// for task targets.
	Version int
}

// AuditListOpts filters the audit list. All fields are optional;
// zero values mean no filter. Filters combine with AND.
type AuditListOpts struct {
	// Target filters to audits on this document path or task key.
	Target string

	// ByAuthor filters to audits created by this author.
	ByAuthor string

	// Assignee filters to audits assigned to this person.
	Assignee string

	// Status filters to threads with this effective status.
	Status string

	// Pending is a shorthand: effective status in (pending, needs-work).
	Pending bool

	// Since filters to audits created after this time. Zero means
	// no filter.
	Since time.Time
}

// AuditStatusOpts configures the status inbox query.
type AuditStatusOpts struct {
	// Since filters to threads with activity after this time.
	// Zero means no filter.
	Since time.Time
}

// AuditStatus is the agent's inbox - pending audit threads requiring
// the given author's response.
type AuditStatus struct {
	// Author is the identity this status was computed for.
	Author string

	// Pending contains top-level audits for threads requiring this
	// author's response. Each thread has been evaluated: only those
	// where the last entry is NOT from this author are included.
	Pending []Audit

	// Summary provides aggregate counts for quick display.
	Summary AuditSummary
}

// AuditSummary provides aggregate counts for the status view.
type AuditSummary struct {
	// Total is the number of pending threads.
	Total int

	// NeedsWork is the count of threads with effective status "needs-work".
	NeedsWork int

	// Pending is the count of threads with effective status "pending".
	Pending int
}
