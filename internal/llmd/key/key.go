// Package key provides identifier generation for documents and
// entities.
//
// A key is a 10-character base36 string. Its value is the time in
// milliseconds plus a counter, multiplied by 1024, plus a stamp drawn
// at random once per process. Keys therefore sort by creation time,
// never repeat within a process (the counter), and two processes
// clash only when they hit the same millisecond and the same stamp:
// about one in a million per pair of keys. The store is the only place
// that can tell, so every insert goes through Insert, which asks for a
// fresh key when the store refuses one as taken.
//
// 36^10 is about 3.6e15; the time component multiplied by 1024 stays
// under that until the year 2083.
package key

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Length is the number of characters in every key.
const Length = 10

const padding = "0000000000"

// stamps is how many per-process stamps there are; a key's low 10
// bits hold the process's stamp.
const stamps = 1024

var (
	ErrInvalidLength = errors.New("key must be 10 characters")
	ErrInvalidChar   = errors.New("key must be lowercase base36")
)

// counter keeps keys apart within a process, however fast they are
// made. It starts at a random offset so two processes started in the
// same millisecond still differ in the time component.
var counter atomic.Int64

// stamp tells this process's keys from another's when both land on the
// same time component.
var stamp int64

func init() {
	counter.Store(rand.Int64N(1000))
	stamp = rand.Int64N(stamps)
}

// Generate returns a new 10-character base36 key.
func Generate() string {
	if k, ok := forced.next(); ok {
		return k
	}
	ms := time.Now().UnixMilli()
	c := counter.Add(1)
	return format((ms+c)*stamps + stamp)
}

// forced holds keys that Generate must return first, for tests that
// need a clash.
var forced forcedKeys

type forcedKeys struct {
	sync.Mutex
	keys []string
}

func (f *forcedKeys) next() (string, bool) {
	f.Lock()
	defer f.Unlock()
	if len(f.keys) == 0 {
		return "", false
	}
	k := f.keys[0]
	f.keys = f.keys[1:]
	return k, true
}

// Force makes the next calls to Generate return keys in order before
// normal generation resumes. Tests use it to make a key clash happen;
// calling it with no keys clears any left over.
func Force(keys ...string) {
	forced.Lock()
	defer forced.Unlock()
	forced.keys = append([]string(nil), keys...)
}

// attempts is how many fresh keys Insert tries before giving up. A
// clash is rare, two in a row would mean something other than chance.
const attempts = 5

// Insert runs insert with a fresh key and returns the key it stored.
// When the store refuses the row because column, such as "tasks.key",
// already holds that key, Insert generates another key and tries
// again. Any other error is returned as is.
func Insert(column string, insert func(k string) error) (string, error) {
	var err error
	for range attempts {
		k := Generate()
		err = insert(k)
		if err == nil {
			return k, nil
		}
		if !Taken(err, column) {
			return "", err
		}
	}
	return "", fmt.Errorf("%d keys in a row were already taken: %w", attempts, err)
}

// Taken reports whether err is SQLite refusing a row because column
// already holds that value, as in "UNIQUE constraint failed: tasks.key".
func Taken(err error, column string) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: "+column)
}

// GenerateAt returns the key for a millisecond timestamp with a zero
// stamp, so keys made this way sort with those from Generate.
func GenerateAt(ms int64) string {
	return format(ms * stamps)
}

// format renders a key value as 10 lowercase base36 characters.
func format(v int64) string {
	s := strconv.FormatInt(v, 36)
	if len(s) >= Length {
		return s
	}
	return padding[:Length-len(s)] + s
}

// Validate checks that a key is a valid 10-character base36 string.
func Validate(k string) error {
	if len(k) != Length {
		return ErrInvalidLength
	}
	for _, c := range k {
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') {
			return ErrInvalidChar
		}
	}
	return nil
}
