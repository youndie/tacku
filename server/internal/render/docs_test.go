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
		Stages: []string{"one"},
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
	shot.Stages = append(shot.Stages, "two")
	shot.Items = append(shot.Items, docsboard.Item{ID: "B-03", Number: 3, Title: "t", Status: "done", Stage: "two"})

	if strings.Contains(drawn(t, render.DocsBoard{Snapshot: shot, Now: time.Now()}), `"two"`) {
		t.Error("этап без открытых задач занял колонку — на живом источнике таких больше, чем работающих")
	}
}

// Both rules here were written by looking at a real item rather than at the fixture, and neither
// was visible in the fixture at all.
func TestAnItemKeepsTheShapeItWasWrittenIn(t *testing.T) {
	item := docsboard.Item{
		ID: "B-171", Title: "Rules are off by name", Status: "open", Path: "backlog/B-171-x.md",
		Body: "# B-171 — Rules are off by name\n\nOne paragraph\nover two lines.\n\n" +
			"| Rule | Places |\n|---|---|\n| naming | 42 |\n",
	}
	tree, err := json.Marshal(render.DocsItem{Item: item}.Screen())
	if err != nil {
		t.Fatal(err)
	}

	var texts []string
	for _, line := range strings.Split(string(tree), `"text":"`)[1:] {
		texts = append(texts, line[:strings.Index(line, `"`)])
	}

	// Заголовок файла повторяет заголовок задачи, который уже стоит наверху экрана: шаблон метода
	// открывает каждую задачу строкой `# B-NN — <title>`.
	for _, text := range texts {
		if strings.HasPrefix(text, "B-171 —") {
			t.Errorf("собственный заголовок файла нарисован вторым заголовком: %q", text)
		}
	}
	// Строка таблицы — это строка. Склеенная в абзац с соседними, таблица превращается в одно
	// нечитаемое предложение, что и произошло на живой задаче.
	rows := 0
	for _, text := range texts {
		if strings.HasPrefix(text, "| ") {
			rows++
		}
		if strings.Count(text, "|") > 3 {
			t.Errorf("строки таблицы склеились: %q", text)
		}
	}
	if rows != 2 {
		t.Errorf("строк таблицы %d, а их две", rows)
	}
	joined := false
	for _, text := range texts {
		joined = joined || text == "One paragraph over two lines."
	}
	if !joined {
		t.Errorf("абзац не собрался из двух строк: %v", texts)
	}
}
