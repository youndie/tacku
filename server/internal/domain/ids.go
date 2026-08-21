package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// MemberID identifies a person or an agent account inside a workspace.
type MemberID string

// BoardID identifies a board.
type BoardID string

// TaskID is the identifier a person says out loud: TAC-124.
//
// It is minted here rather than taken from the store. A row key on the wire leaks how the storage
// is arranged and pins the product to it; and an agent carries this string in its context between
// tool calls and quotes it back to a person, so it has to be short and pronounceable.
//
// It is deliberately sequential and therefore enumerable. That is a real property and not an
// oversight: knowing TAC-124 exists tells you TAC-123 probably does too. Authorisation is per
// workspace, so guessing a neighbour buys nothing — but if tasks ever become shareable by link,
// this is the decision to revisit first.
type TaskID string

const taskPrefix = "TAC-"

var taskIDPattern = regexp.MustCompile(`^TAC-[1-9][0-9]*$`)

// NewTaskID builds the identifier for the n-th task of a workspace.
func NewTaskID(n int64) (TaskID, error) {
	if n < 1 {
		return "", fmt.Errorf("domain: task number must be positive, got %d", n)
	}
	return TaskID(taskPrefix + strconv.FormatInt(n, 10)), nil
}

// Number returns the sequence number behind an identifier.
func (id TaskID) Number() (int64, error) {
	if !id.Valid() {
		return 0, fmt.Errorf("domain: %q is not a task identifier", string(id))
	}
	return strconv.ParseInt(strings.TrimPrefix(string(id), taskPrefix), 10, 64)
}

// String is the identifier as a person says it.
func (id TaskID) String() string { return string(id) }

// Valid reports whether the identifier has the shape this workspace mints.
//
// Checked rather than assumed because this string arrives from an agent, and a model that has been
// told about TAC-124 will cheerfully invent TAC-0 or "TAC-124 " when it needs one.
func (id TaskID) Valid() bool { return taskIDPattern.MatchString(string(id)) }
