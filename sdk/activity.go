package sdk

// ActivityStore provides the unified activity feed across all domains.
// Events come from documents (writes, deletes), entities (tags, links),
// and tasks (board changes).
type ActivityStore interface {
	// Recent returns the most recent events across all domains,
	// sorted by timestamp descending. Limit 0 means all events.
	Recent(limit int) ([]Activity, error)
}

// Activity represents a single event in the unified activity feed.
type Activity struct {
	Type      string // "document", "tag", "link", "task"
	Action    string // "written", "deleted", "tagged", "untagged", "linked", "unlinked", "created", "moved", etc.
	Subject   string // document path or task key
	Author    string
	Detail    string // version message, tag name, link target, old→new
	Timestamp int64  // Unix milliseconds
}
