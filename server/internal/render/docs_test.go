package render_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/docsboard"
	"github.com/youndie/tacku/server/internal/render"
)

func snapshot() docsboard.Snapshot {
	return docsboard.Snapshot{
		Title:  "A backlog",
		Stages: []docsboard.Stage{{ID: "one", Title: "One"}},
		Items: []docsboard.Item{
			{ID: "B-01", Number: 1, Title: "Open one", Status: "open", Stage: "one"},
			{ID: "B-02", Number: 2, Title: "Finished one", Status: "done", Stage: "one"},
		},
		Head:    "abc",
		TakenAt: time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC),
	}
}

func drawn(t *testing.T, board render.DocsBoard) string {
	t.Helper()
	body, err := json.Marshal(board.Screen())
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// A list takes the height that is left, so a node written after it starts below the bottom of the
// screen. The tally was there for one build and nobody could see it; the tree said nothing and the
// screenshot did.
func TestTheTallyStandsAboveTheListAndNotBelowIt(t *testing.T) {
	tree := drawn(t, render.DocsBoard{Snapshot: snapshot(), Now: time.Now()})

	tally := strings.Index(tree, `"docs-column-one-done"`)
	list := strings.Index(tree, `"docs-column-one-list"`)
	if tally < 0 {
		t.Fatal("колонка не сказала, сколько в ней сделано")
	}
	if tally > list {
		t.Error("строка «сделано» стоит после списка — то есть за нижним краем экрана")
	}
}

func TestAFinishedStageIsNotDrawnAsAnEmptyColumn(t *testing.T) {
	shot := snapshot()
	shot.Stages = append(shot.Stages, docsboard.Stage{ID: "two", Title: "Nothing left here"})
	shot.Items = append(shot.Items, docsboard.Item{ID: "B-03", Number: 3, Title: "t", Status: "done", Stage: "two"})

	if strings.Contains(drawn(t, render.DocsBoard{Snapshot: shot, Now: time.Now()}), "Nothing left here") {
		t.Error("этап без открытых задач занял колонку — на живом источнике таких больше, чем работающих")
	}
}
