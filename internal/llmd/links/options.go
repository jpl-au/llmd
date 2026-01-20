package links

import "github.com/jpl-au/llmd/internal/llmd/core"

// Direction specifies which links to return.
type Direction int

const (
	Outgoing Direction = iota + 1 // links from this document
	Incoming                      // links to this document
	Both                          // all links involving this document
)

// Options configures link operations.
type Options struct {
	core.WriteContext
	Label     string    // link label (e.g., "related", "depends-on")
	Direction Direction // for List: which direction to query
}
