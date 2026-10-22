// Package clock is the store's single source of wall-clock time.
// Production reads time.Now; tests install a deterministic clock through
// clocktest so timestamp-ordered behaviour can be asserted without
// sleeping between writes.
package clock

import "time"

// Now returns the current time in Unix milliseconds. It is a variable so
// tests can replace it; store code must call it rather than time.Now.
var Now = func() int64 { return time.Now().UnixMilli() }
