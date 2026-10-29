// order.go defines how tasks are ordered within a column.

package tasks

import (
	"cmp"
	"errors"
	"math"

	"github.com/jpl-au/llmd/internal/errs"

	"github.com/jpl-au/llmd/pkg/model/task"
)

// Order is how tasks are ordered within a column, for the board,
// review and Claim alike, so what a worker takes next is always the
// top of what a person sees.
type Order string

const (
	// OrderPriority puts tasks with a priority first, lowest number
	// first (1 is the highest priority), then the rest; board position
	// breaks ties. The default.
	OrderPriority Order = "priority"

	// OrderPosition is board position only. Priority is a label.
	OrderPosition Order = "position"
)

// ErrInvalidPriority is returned for a negative priority. Zero means
// none; 1 is the highest; larger numbers are lower.
var ErrInvalidPriority = errors.New("a priority must be 0 for none, or 1 or more, 1 being the highest.")

// invalidPriority is ErrInvalidPriority naming the rejected value.
func invalidPriority(p int) error {
	return errs.New(ErrInvalidPriority, "%d is not a valid priority. Use 0 for none, or 1 or more, where 1 is the highest.", p)
}

// ErrInvalidOrder is returned by SetOrder for an unknown order.
var ErrInvalidOrder = errors.New("task.order must be priority or position.")

// SetOrder sets the ordering within columns. The zero value means
// OrderPriority.
func (t *Tasks) SetOrder(o Order) error {
	switch o {
	case "", OrderPriority, OrderPosition:
		t.order = o
		return nil
	}
	return ErrInvalidOrder
}

// byPriority reports whether priority takes part in ordering.
func (t *Tasks) byPriority() bool {
	return t.order == "" || t.order == OrderPriority
}

// priorityRank is the sort key for a priority: lower is earlier, and
// no priority sorts after every priority.
func priorityRank(p int) int {
	if p <= 0 {
		return math.MaxInt
	}
	return p
}

// compare orders two tasks of the same column.
func (t *Tasks) compare(a, b *task.Task) int {
	if t.byPriority() {
		if c := cmp.Compare(priorityRank(a.Priority), priorityRank(b.Priority)); c != 0 {
			return c
		}
	}
	if c := cmp.Compare(a.Position, b.Position); c != 0 {
		return c
	}
	if c := cmp.Compare(a.CreatedAt, b.CreatedAt); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}

// orderBy is the SQL ORDER BY clause matching compare, for queries
// that read one column.
func (t *Tasks) orderBy() string {
	if t.byPriority() {
		return "ORDER BY CASE WHEN priority > 0 THEN priority ELSE 9223372036854775807 END ASC, position ASC, created_at ASC, id ASC"
	}
	return "ORDER BY position ASC, created_at ASC, id ASC"
}
