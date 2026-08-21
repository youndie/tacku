package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/store/sqlite"
)

const (
	anna  = domain.MemberID("anna")
	ivan  = domain.MemberID("ivan")
	robot = domain.MemberID("anna-agent")
)

func open(t *testing.T) (*sqlite.Store, context.Context) {
	t.Helper()
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "tacku.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, context.Background()
}

func board(t *testing.T, s *sqlite.Store, ctx context.Context) domain.Board {
	t.Helper()
	b, err := s.CreateBoard(ctx, "Sprint 24")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func task(t *testing.T, s *sqlite.Store, ctx context.Context, b domain.Board, title string) domain.Task {
	t.Helper()
	task, err := s.CreateTask(ctx, domain.Task{Board: b.ID, Title: title}, domain.Human(anna))
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestEveryChangeLandsInTheJournalWithBothActors(t *testing.T) {
	s, ctx := open(t)
	b := board(t, s, ctx)

	created := task(t, s, ctx, b, "Fix login redirect loop")
	if _, err := s.MoveTask(ctx, created.ID, domain.StatusInReview, domain.Agent(robot, "0.1.0", anna)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Comment(ctx, created.ID, "Reproduced on staging.", domain.Human(ivan)); err != nil {
		t.Fatal(err)
	}

	changes, _, err := s.Changes(ctx, domain.Start, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 {
		t.Fatalf("journal holds %d entries, want 3 (created, moved, commented)", len(changes))
	}

	move := changes[1]
	if move.Kind != domain.ChangeStatusMoved {
		t.Errorf("second entry is %q, want %q", move.Kind, domain.ChangeStatusMoved)
	}
	if move.From != string(domain.StatusTodo) || move.To != string(domain.StatusInReview) {
		t.Errorf("move recorded %q -> %q", move.From, move.To)
	}
	if !move.By.ByAgent() {
		t.Error("the agent's move is not marked as one; the feed cannot tell it from a person's")
	}
	if move.By.Executor.Member != robot || move.By.OnBehalfOf != anna {
		t.Errorf("provenance is %v acting for %q, want %q acting for %q",
			move.By.Executor.Member, move.By.OnBehalfOf, robot, anna)
	}
	if move.By.Executor.Version != "0.1.0" {
		t.Error("the agent's version is missing: a history that cannot say which build acted answers nothing")
	}

	if changes[2].By.ByAgent() {
		t.Error("a person's comment is recorded as an agent's")
	}
}

// A feed exists to say what changed. An entry saying a task moved from In review to In review is
// not a small inaccuracy — it is the feed filling itself with nothing, which an agent retrying a
// call would do all by itself.
func TestARepeatedMoveWritesNothing(t *testing.T) {
	s, ctx := open(t)
	b := board(t, s, ctx)
	created := task(t, s, ctx, b, "Audit the session cookie flags")

	by := domain.Agent(robot, "0.1.0", anna)
	if _, err := s.MoveTask(ctx, created.ID, domain.StatusInProgress, by); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Latest(ctx)

	got, err := s.MoveTask(ctx, created.ID, domain.StatusInProgress, by)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusInProgress {
		t.Errorf("the repeat returned status %q", got.Status)
	}

	after, _ := s.Latest(ctx)
	if before != after {
		t.Errorf("the journal moved from %q to %q on a change that changed nothing", before, after)
	}
}

func TestCursorReturnsExactlyWhatFollowedIt(t *testing.T) {
	s, ctx := open(t)
	b := board(t, s, ctx)

	first := task(t, s, ctx, b, "one")
	mark, err := s.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}

	second := task(t, s, ctx, b, "two")
	if _, err := s.MoveTask(ctx, first.ID, domain.StatusDone, domain.Human(anna)); err != nil {
		t.Fatal(err)
	}

	changes, next, err := s.Changes(ctx, mark, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("got %d changes after the mark, want 2", len(changes))
	}
	if changes[0].Task != second.ID || changes[1].Task != first.ID {
		t.Errorf("changes came back as %v, want the creation of %q then the move of %q",
			[]domain.TaskID{changes[0].Task, changes[1].Task}, second.ID, first.ID)
	}

	empty, _, err := s.Changes(ctx, next, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("reading again from the returned cursor produced %d entries; a reader would see them twice", len(empty))
	}
}

// A limit that advanced the cursor to the end would turn paging into silent data loss, and for an
// agent polling by cursor the loss is invisible.
func TestALimitDoesNotSkipTheRemainder(t *testing.T) {
	s, ctx := open(t)
	b := board(t, s, ctx)
	for _, title := range []string{"one", "two", "three", "four"} {
		task(t, s, ctx, b, title)
	}

	seen := 0
	cursor := domain.Start
	for range 10 {
		page, next, err := s.Changes(ctx, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		seen += len(page)
		cursor = next
	}
	if seen != 4 {
		t.Errorf("walking in pages of two saw %d of 4 entries", seen)
	}
}

// A fabricated cursor read as zero would replay the whole journal into an agent's context, which is
// expensive and looks like the agent losing its place rather than like an error.
func TestAForeignCursorIsRefused(t *testing.T) {
	s, ctx := open(t)

	for _, bad := range []domain.Cursor{"", "17", "cabc", "c-1", "seq:4"} {
		if _, _, err := s.Changes(ctx, bad, 10); !errors.Is(err, domain.ErrBadCursor) {
			t.Errorf("cursor %q was accepted (err = %v)", string(bad), err)
		}
	}
}

func TestStateAndCursorSurviveARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tacku.db")
	ctx := context.Background()

	first, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := first.CreateBoard(ctx, "Sprint 24")
	if err != nil {
		t.Fatal(err)
	}
	created, err := first.CreateTask(ctx, domain.Task{Board: b.ID, Title: "Ship the settings redesign"}, domain.Human(anna))
	if err != nil {
		t.Fatal(err)
	}
	mark, err := first.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.MoveTask(ctx, created.ID, domain.StatusInReview, domain.Agent(robot, "0.1.0", anna)); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	task, err := second.Task(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.StatusInReview {
		t.Errorf("after restart the task is %q, want %q", task.Status, domain.StatusInReview)
	}

	changes, _, err := second.Changes(ctx, mark, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Kind != domain.ChangeStatusMoved {
		t.Fatalf("the cursor taken before the restart returned %d entries, want the one move", len(changes))
	}
	if !changes[0].By.ByAgent() {
		t.Error("provenance did not survive the restart")
	}

	// The next task must not reuse a number: an identifier people quote has to stay unique across
	// restarts, and a counter that resets is the classic way it does not.
	next, err := second.CreateTask(ctx, domain.Task{Board: b.ID, Title: "another"}, domain.Human(anna))
	if err != nil {
		t.Fatal(err)
	}
	if next.ID == created.ID {
		t.Errorf("the task number restarted: %q was handed out twice", string(next.ID))
	}
}

// Two parallel writers taking the same number is a failure already paid for in a neighbouring
// project, where a merge joined two branches that had each claimed one.
func TestParallelCreatesNeverShareANumber(t *testing.T) {
	s, ctx := open(t)
	b := board(t, s, ctx)

	const writers = 12
	var wg sync.WaitGroup
	ids := make([]domain.TaskID, writers)
	errs := make([]error, writers)

	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := s.CreateTask(ctx, domain.Task{Board: b.ID, Title: "parallel"}, domain.Human(anna))
			ids[i], errs[i] = task.ID, err
		}()
	}
	wg.Wait()

	seen := map[domain.TaskID]bool{}
	for i := range writers {
		if errs[i] != nil {
			t.Fatalf("writer %d failed: %v", i, errs[i])
		}
		if seen[ids[i]] {
			t.Fatalf("identifier %q was handed out twice", string(ids[i]))
		}
		seen[ids[i]] = true
	}
}

func TestUnknownStatusAndTaskAreRefused(t *testing.T) {
	s, ctx := open(t)
	b := board(t, s, ctx)
	created := task(t, s, ctx, b, "Tighten the empty states copy")

	if _, err := s.MoveTask(ctx, created.ID, "archived", domain.Human(anna)); !errors.Is(err, domain.ErrInvalidTask) {
		t.Errorf("a status outside the closed set was accepted (err = %v)", err)
	}
	if _, err := s.MoveTask(ctx, "TAC-9999", domain.StatusDone, domain.Human(anna)); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("moving a task that does not exist returned %v, want not found", err)
	}
	if _, err := s.MoveTask(ctx, "tac-1", domain.StatusDone, domain.Human(anna)); !errors.Is(err, domain.ErrInvalidTask) {
		t.Errorf("a malformed identifier was accepted (err = %v)", err)
	}
}

func TestAgentWithoutAPrincipalIsRefused(t *testing.T) {
	s, ctx := open(t)
	b := board(t, s, ctx)

	bad := []domain.Provenance{
		{Executor: domain.Actor{Kind: domain.ActorAgent, Member: robot, Version: "0.1.0"}},
		{Executor: domain.Actor{Kind: domain.ActorAgent, Member: robot}, OnBehalfOf: anna},
		{Executor: domain.Actor{Kind: domain.ActorHuman, Member: anna}, OnBehalfOf: ivan},
		{Executor: domain.Actor{Kind: "robot", Member: robot}, OnBehalfOf: anna},
	}
	for i, by := range bad {
		if _, err := s.CreateTask(ctx, domain.Task{Board: b.ID, Title: "x"}, by); !errors.Is(err, domain.ErrInvalidProvenance) {
			t.Errorf("provenance %d was accepted (err = %v)", i, err)
		}
	}
}

// Who touched a task last is a question with no page size.
//
// This used to be answered by reading the first five hundred journal entries and keeping the last
// one seen per task. Correct until a team writes the five hundred and first, and then wrong in the
// quietest way available: the newest entries stop being read, so the provenance stripe fades off
// the busiest boards first while every test on a small fixture stays green.
func TestTheLastActorIsFoundBeyondAnyPage(t *testing.T) {
	s, ctx := open(t)
	b := board(t, s, ctx)

	subject, err := s.CreateTask(ctx, domain.Task{Board: b.ID, Title: "watched"}, domain.Human(anna))
	if err != nil {
		t.Fatal(err)
	}

	// Enough noise to bury the entry that matters under any bound a reader might have picked.
	noise, err := s.CreateTask(ctx, domain.Task{Board: b.ID, Title: "noisy"}, domain.Human(anna))
	if err != nil {
		t.Fatal(err)
	}
	for i := range 600 {
		if _, err := s.Comment(ctx, noise.ID, "chatter", domain.Human(ivan)); err != nil {
			t.Fatalf("comment %d: %v", i, err)
		}
	}

	if _, err := s.Comment(ctx, subject.ID, "an agent had the last word", domain.Agent(robot, "0.1.0", anna)); err != nil {
		t.Fatal(err)
	}

	actors, err := s.LastActors(ctx)
	if err != nil {
		t.Fatal(err)
	}

	last, ok := actors[subject.ID]
	if !ok {
		t.Fatalf("no last actor for %s at all", subject.ID)
	}
	if !last.ByAgent() {
		t.Errorf("the last word on %s was an agent's and the answer names %q — the newest entry was not read",
			subject.ID, last.Executor.Member)
	}
	if last.OnBehalfOf != anna {
		t.Errorf("on behalf of %q, want %q", last.OnBehalfOf, anna)
	}

	// The neighbour keeps its own answer, so this is not a check that everything looks like an agent.
	if noisy := actors[noise.ID]; noisy.ByAgent() {
		t.Error("the noisy task reports an agent, and nothing an agent did touched it")
	}
}

// "Read up to here" has to outlive the process.
//
// The item asks for a boundary that survives a restart of the application, and the whole point of
// the catch-up screen rests on it: a boundary kept in memory would reset to the beginning of time
// every deploy, and the screen would greet everyone with the entire history of the workspace and
// call it news.
func TestTheReadBoundarySurvivesAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tacku.db")

	first, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	b := board(t, first, ctx)
	for range 3 {
		if _, err := first.CreateTask(ctx, domain.Task{Board: b.ID, Title: "task"}, domain.Human(anna)); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := first.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if latest == domain.Start {
		t.Fatal("nothing was written, so a boundary at the end proves nothing")
	}
	if err := first.MarkSeen(ctx, anna, latest); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })

	after, err := second.SeenAt(ctx, anna)
	if err != nil {
		t.Fatal(err)
	}
	if after != latest {
		t.Fatalf("the boundary came back as %q, not %q", after, latest)
	}

	// And it means what it says: nothing stands after it until something new happens, and then
	// exactly that one thing does.
	changes, _, err := second.Changes(ctx, after, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("%d changes stand after a boundary that was set at the end", len(changes))
	}
	if _, err := second.CreateTask(ctx, domain.Task{Board: b.ID, Title: "filed later"}, domain.Human(ivan)); err != nil {
		t.Fatal(err)
	}
	changes, _, err = second.Changes(ctx, after, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("after one new task the feed would show %d changes, want 1", len(changes))
	}
}
