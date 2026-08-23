// Package docsboard reads a docs-as-code backlog out of a git repository and shows it as a board.
//
// It reads and never writes, and that is the whole design rather than a stage of it. The repository
// is the tracker: an item is a file, its status is frontmatter, it is quoted by identifier from the
// documents around it and it changes inside a reviewed pull request. A view that could also write
// would immediately owe an answer to "which of the two is right", and there is no cheap one. So the
// board here is a window, the source is unaware it is being looked at, and nothing in this package
// takes a Provenance — see docs/backlog/B-53 and B-30.
package docsboard

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Item is one backlog item as its file declares it.
//
// Every field but the number is kept as written rather than mapped onto a vocabulary of ours. The
// source repositories run their own copy of the index generator and their own house rules, and this
// was measured on a live one rather than imagined: a status the method's specification does not
// list, a size spelled `S/M`, a priority that is a word instead of a letter and a digit. A parser
// that accepted only the documented set would either drop such an item or file it under a
// neighbouring value, and the second is worse than the first — the board would look complete.
type Item struct {
	ID        string
	Number    int
	Title     string
	Status    string
	Priority  string
	Size      string
	Stage     string
	Epic      string
	BlockedBy []string

	// Path is where the file sits inside the repository, so that a card can lead to it. Kept from
	// the archive rather than rebuilt from the identifier: the item file name carries a slug this
	// package has no way to reconstruct.
	Path string
}

// Done reports whether the item is finished, which is the one status this package acts on.
//
// The rest are shown as the word the file uses. `dropped` is the reason it works this way: the
// method's own generator leaves a dropped item out of the open list, while a repository running an
// older copy of that generator counts it in — two generators disagreeing about one word. A view
// that picked a side would be a third opinion, expressed by making an item disappear.
func (i Item) Done() bool { return i.Status == "done" }

var (
	frontmatterBlock = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---`)
	itemID           = regexp.MustCompile(`^B-([0-9]+)$`)

	// itemFile is what the method calls an item: the number and a slug. Matched by name because the
	// directory also holds the things a documentation tree collects around a backlog — a README, a
	// template — and reading one of those as an item would fail the whole load.
	itemFile = regexp.MustCompile(`^B-[0-9]+-.*\.md$`)
)

// ParseItem reads one item file.
//
// The frontmatter subset understood here is the one the method's template writes: `key: value`, a
// value optionally in double quotes, and a flow list for `blocked_by`. Keys that are not recognised
// are ignored rather than refused — a repository is free to carry fields of its own, and a board
// that fell over on the first unknown one would be a board nobody could point at their repository.
func ParseItem(path, text string) (Item, error) {
	block := frontmatterBlock.FindStringSubmatch(text)
	if block == nil {
		return Item{}, fmt.Errorf("docsboard: %s has no frontmatter", path)
	}

	item := Item{Path: path}
	for _, line := range strings.Split(block[1], "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		value = unquote(strings.TrimSpace(value))

		switch strings.TrimSpace(key) {
		case "id":
			item.ID = value
		case "title":
			item.Title = value
		case "status":
			item.Status = value
		case "priority":
			item.Priority = value
		case "size":
			item.Size = value
		case "stage":
			item.Stage = value
		case "epic":
			item.Epic = value
		case "blocked_by":
			item.BlockedBy = list(value)
		}
	}

	if item.ID == "" {
		return Item{}, fmt.Errorf("docsboard: %s declares no id", path)
	}
	number := itemID.FindStringSubmatch(item.ID)
	if number == nil {
		return Item{}, fmt.Errorf("docsboard: %s declares id %q, which is not B-<number>", path, item.ID)
	}
	// Parsed and kept, because the identifier sorts as text and the board must not: with a hundred
	// items in a repository B-100 stands between B-10 and B-11, and a column ordered that way looks
	// ordered.
	item.Number, _ = strconv.Atoi(number[1])

	return item, nil
}

func unquote(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
		return value[1 : len(value)-1]
	}
	return value
}

func list(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		if value == "" || value == "-" {
			return nil
		}
		return []string{value}
	}
	var out []string
	for _, part := range strings.Split(value[1:len(value)-1], ",") {
		if part = unquote(strings.TrimSpace(part)); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// Stage is one column of the board.
type Stage struct {
	ID    string
	Title string
}

// NoStage is the column items land in when their file names none.
const NoStage = ""

// stageRow matches a row of the hand-written stage table in the index: `| stage-id | name | ... |`.
//
// The same expression the method's generator uses, and it is deliberately loose enough to match
// rows of any other two-column table in the same file. That is why the result is filtered by the
// stages items actually declare: a repository's index carries deployment tables and decision tables
// whose first cell is a number or a service name, and filtering is what tells them apart without
// this package having to guess at the document's structure.
var stageRow = regexp.MustCompile("(?m)^\\|\\s*`?([A-Za-z0-9][A-Za-z0-9._-]*)`?\\s*\\|\\s*([^|]*?)\\s*\\|")

// Stages is the column order: the stages the index names, in its order, then the ones it does not.
//
// The second half is the point. A live repository had an item whose stage is declared nowhere in
// the table, and a board built from the table alone would have shown every column correctly and
// silently lost that item. Its column is named by its bare identifier, which is ugly on purpose:
// the ugliness is the report.
func Stages(index string, items []Item) []Stage {
	used := map[string]bool{}
	for _, item := range items {
		if item.Stage != "" {
			used[item.Stage] = true
		}
	}

	var ordered []Stage
	seen := map[string]bool{}
	for _, row := range stageRow.FindAllStringSubmatch(index, -1) {
		id, title := row[1], strings.TrimSpace(row[2])
		if !used[id] || seen[id] {
			continue
		}
		seen[id] = true
		if title == "" {
			title = id
		}
		ordered = append(ordered, Stage{ID: id, Title: title})
	}

	var undeclared []string
	for id := range used {
		if !seen[id] {
			undeclared = append(undeclared, id)
		}
	}
	sort.Strings(undeclared)
	for _, id := range undeclared {
		ordered = append(ordered, Stage{ID: id, Title: id})
	}

	for _, item := range items {
		if item.Stage == "" {
			ordered = append(ordered, Stage{ID: NoStage, Title: "No stage"})
			break
		}
	}
	return ordered
}

// priorityRank orders the priorities the method names, and puts everything else last.
//
// Unknown last rather than dropped or renamed: a priority this build has never heard of is still a
// priority somebody wrote down, and the item keeps its place on the board with the word it carries.
func priorityRank(priority string) int {
	switch priority {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	case "infra":
		return 4
	}
	return 5
}

// Column is the unfinished work of one stage, in the order a person reads it: by priority, then by
// number.
func Column(items []Item, stage string) []Item {
	var out []Item
	for _, item := range items {
		if item.Stage == stage && !item.Done() {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if ra, rb := priorityRank(out[a].Priority), priorityRank(out[b].Priority); ra != rb {
			return ra < rb
		}
		return out[a].Number < out[b].Number
	})
	return out
}

// DoneCount is how much of a stage is already finished.
//
// Counted rather than listed: a repository that has been running for a while is mostly done items,
// and a board that draws them all is a wall a person scrolls past to reach the fourteen that
// matter. The number stays because a column with nothing open and a column that never existed are
// not the same thing.
func DoneCount(items []Item, stage string) int {
	n := 0
	for _, item := range items {
		if item.Stage == stage && item.Done() {
			n++
		}
	}
	return n
}
