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
	Type      string `json:"type"`    // "document", "tag", "link", "task"
	Action    string `json:"action"`  // "written", "deleted", "tagged", "untagged", "linked", "unlinked", "created", "moved", etc.
	Subject   string `json:"subject"` // document path or task key
	Author    string `json:"author"`
	Detail    string `json:"detail"`    // version message, tag name, link target, old→new
	Timestamp int64  `json:"timestamp"` // Unix milliseconds
}
