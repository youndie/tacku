package docsboard

import (
	"strings"
	"testing"
)

func item(t *testing.T, body string) Item {
	t.Helper()
	parsed, err := ParseItem("backlog/B-01-x.md", body)
	if err != nil {
		t.Fatalf("не разобралось: %v", err)
	}
	return parsed
}

// The vocabulary of a live repository is wider than the one the method's specification lists, and
// this is the case that decides whether the board can be pointed at a real one at all.
func TestAWordOutsideTheMethodSurvives(t *testing.T) {
	parsed := item(t, `---
id: B-01
title: "Ship it"
status: paused
priority: infra
size: S/M
stage: beta
---
`)

	if parsed.Status != "paused" {
		t.Errorf("статус %q, а записан был paused", parsed.Status)
	}
	if parsed.Priority != "infra" || parsed.Size != "S/M" {
		t.Errorf("приоритет %q, размер %q", parsed.Priority, parsed.Size)
	}
	if parsed.Done() {
		t.Error("незнакомый статус посчитан завершением — такая задача исчезнет с доски")
	}
}

func TestATitleKeepsItsColon(t *testing.T) {
	parsed := item(t, "---\nid: B-01\ntitle: \"Storefront: the address is the distribution\"\nstatus: open\n---\n")
	if want := "Storefront: the address is the distribution"; parsed.Title != want {
		t.Errorf("заголовок %q, а был %q", parsed.Title, want)
	}
}

func TestBlockedByIsAList(t *testing.T) {
	parsed := item(t, "---\nid: B-05\ntitle: t\nstatus: open\nblocked_by: [B-02, B-03]\n---\n")
	if len(parsed.BlockedBy) != 2 || parsed.BlockedBy[0] != "B-02" || parsed.BlockedBy[1] != "B-03" {
		t.Errorf("ждёт %v", parsed.BlockedBy)
	}
}

func TestAFileWithoutFrontmatterIsRefused(t *testing.T) {
	if _, err := ParseItem("backlog/B-01-x.md", "# just prose\n"); err == nil {
		t.Fatal("файл без фронтматтера принят молча")
	}
}

func items(t *testing.T, bodies ...string) []Item {
	t.Helper()
	var out []Item
	for _, body := range bodies {
		parsed, err := ParseItem("backlog/x.md", body)
		if err != nil {
			t.Fatalf("не разобралось: %v", err)
		}
		out = append(out, parsed)
	}
	return out
}

func frontmatter(id, status, priority, stage string) string {
	return "---\nid: " + id + "\ntitle: t\nstatus: " + status +
		"\npriority: " + priority + "\nstage: " + stage + "\n---\n"
}

const index = `# Backlog

| Идентификатор | Этап |
|---|---|
| 1 | a deployment table row that is not a stage |
| ` + "`stage-late`" + ` | Late |
| ` + "`stage-early`" + ` | Early |
`

// The finding this exists for: an item naming a stage the index does not declare. Built from the
// table alone the board draws every column correctly and loses that item without a word.
func TestAnUndeclaredStageGetsItsOwnColumn(t *testing.T) {
	parsed := items(t,
		frontmatter("B-01", "open", "P1", "stage-early"),
		frontmatter("B-02", "open", "P1", "platform"),
	)

	stages := Stages(index, parsed)
	if got := strings.Join(stages, ","); got != "stage-early,platform" {
		t.Fatalf("колонки %q: этап без объявления должен встать последним и не пропасть", got)
	}
	if len(Column(parsed, "platform")) != 1 {
		t.Error("задача необъявленного этапа не попала ни в одну колонку")
	}
}

func TestARowThatIsNotAStageIsNotAColumn(t *testing.T) {
	for _, stage := range Stages(index, items(t, frontmatter("B-01", "open", "P1", "stage-early"))) {
		if stage == "1" {
			t.Fatal("строка чужой таблицы стала колонкой")
		}
	}
}

func TestTheDeclaredOrderIsTheIndexOrder(t *testing.T) {
	parsed := items(t,
		frontmatter("B-01", "open", "P1", "stage-early"),
		frontmatter("B-02", "open", "P1", "stage-late"),
	)
	stages := Stages(index, parsed)
	if len(stages) != 2 || stages[0] != "stage-late" || stages[1] != "stage-early" {
		t.Fatalf("порядок колонок взят не из индекса: %v", stages)
	}
}

func TestAnItemWithNoStageStillHasSomewhereToStand(t *testing.T) {
	parsed := items(t, "---\nid: B-09\ntitle: t\nstatus: open\n---\n")
	stages := Stages(index, parsed)
	if len(stages) != 1 || stages[0] != NoStage {
		t.Fatalf("задача без этапа осталась без колонки: %v", stages)
	}
}

// A hundred items is where a repository stops being a toy, and it is also where an identifier
// sorted as text starts lying: B-100 lands between B-10 and B-11.
func TestAColumnIsOrderedByNumberAndNotByText(t *testing.T) {
	parsed := items(t,
		frontmatter("B-100", "open", "P2", "s"),
		frontmatter("B-11", "open", "P2", "s"),
		frontmatter("B-9", "open", "P0", "s"),
	)

	column := Column(parsed, "s")
	var order []string
	for _, one := range column {
		order = append(order, one.ID)
	}
	if got := strings.Join(order, ","); got != "B-9,B-11,B-100" {
		t.Fatalf("порядок %q: сначала приоритет, потом номер как число", got)
	}
}

func TestFinishedWorkIsCountedAndNotDrawn(t *testing.T) {
	parsed := items(t,
		frontmatter("B-01", "done", "P1", "s"),
		frontmatter("B-02", "done", "P1", "s"),
		frontmatter("B-03", "open", "P1", "s"),
	)
	if n := len(Column(parsed, "s")); n != 1 {
		t.Errorf("в колонке %d карточек, а открыта одна", n)
	}
	if n := DoneCount(parsed, "s"); n != 2 {
		t.Errorf("сделано посчитано как %d", n)
	}
}

// Two generators disagree about `dropped`: the method's own leaves it out of the open list, an
// older copy running in a repository counts it in. The board does not cast the deciding vote by
// making the item vanish.
func TestADroppedItemIsStillOnTheBoard(t *testing.T) {
	parsed := items(t, frontmatter("B-04", "dropped", "P3", "s"))
	if len(Column(parsed, "s")) != 1 {
		t.Fatal("снятая задача исчезла — это третье мнение в чужом споре")
	}
}
