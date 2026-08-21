package httpsrv

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/forms"
	"github.com/youndie/tacku/server/internal/render"
)

const myTasksFormID = "my_tasks"

// taskPageSize is small on purpose.
//
// A page large enough to hold everything makes the walk terminate on the first request and proves
// nothing about termination — the interesting path is a full page followed by a short one, and it
// only exists if pages fill.
const taskPageSize = 5

// tasksFilter is the query the list is read through, and the names are the field identifiers of the
// form on the same screen.
//
// That the names are the field identifiers is an assumption rather than a rule: §8.4 says the client
// resends the values as query parameters and does not say under what names. Recorded as Q-04, and it
// will show itself the first time a real client filters and nothing happens.
type tasksFilter struct {
	Status   domain.Status
	Assignee domain.MemberID
	After    int64
}

func filterOf(r *http.Request) tasksFilter {
	query := r.URL.Query()
	after, _ := strconv.ParseInt(query.Get("after"), 10, 64)
	return tasksFilter{
		Status:   domain.Status(query.Get("status")),
		Assignee: domain.MemberID(query.Get("assignee")),
		After:    after,
	}
}

func (f tasksFilter) query() url.Values {
	values := url.Values{}
	if f.Status != "" {
		values.Set("status", string(f.Status))
	}
	if f.Assignee != "" {
		values.Set("assignee", string(f.Assignee))
	}
	return values
}

// keep decides whether a task is in this list.
func (f tasksFilter) keep(task domain.Task) bool {
	if f.Status != "" && task.Status != f.Status {
		return false
	}
	if f.Assignee != "" && task.Assignee != f.Assignee {
		return false
	}
	return true
}

// page returns one page and the number to continue after.
//
// Continuation is by task number rather than by offset. An offset shifts under a list that changes
// while it is being walked — and this list changes while it is being walked by definition, an agent
// filing tasks being the point of the product.
func (f tasksFilter) page(tasks []domain.Task) ([]domain.Task, int64, bool) {
	kept := make([]domain.Task, 0, taskPageSize)
	var last int64

	for _, task := range tasks {
		number, err := task.ID.Number()
		if err != nil || number <= f.After || !f.keep(task) {
			continue
		}
		kept = append(kept, task)
		last = number
		if len(kept) == taskPageSize {
			// A full page offers a continuation; a short one does not, which is what makes the walk
			// terminate (§8.2). Offering one from a short page is how a client loops for ever over
			// nothing.
			return kept, last, true
		}
	}
	return kept, last, false
}

// myTasks is the filtered list, and it is a form rather than a screen because a filter is an
// ordinary form field (§8.4) — the protocol has no separate mechanism for filtering.
//
// Which is also why the board is not this screen: the board carries no input and is therefore
// cacheable, and putting filters on it would have cost that.
func myTasks(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		filter := filterOf(r)
		if filter.Assignee == "" {
			filter.Assignee = principal.Provenance.OnBehalfOf
		}

		tasks, err := allTasks(r, store)
		if err != nil {
			fail(w, err)
			return
		}
		page, last, more := filter.page(tasks)

		form := forms.New(myTasksFormID)
		status := form.Select("status", "Status", "Any status", append(
			[]render.SelectOption{{ID: "", Label: "Any status"}}, statusOptions()...), nil)

		items := make([]render.Component, 0, len(page))
		for _, task := range page {
			items = append(items, render.TaskRow(task))
		}

		next := ""
		if more {
			next = pageURL(filter, last)
		}

		screen := render.Column("screen-my-tasks", 20,
			[]render.Modifier{render.Padding(32), render.Background(render.ColorSurface)},
			render.Text("my-tasks-title", "My tasks", render.TextDisplay),
			status,
			render.PaginatedList("my-tasks-list", items, next, render.EmptyMyTasks(),
				render.Weight(1)),
		)

		respond(w, r, form.Build(screen))
	}
}

// tasksPage serves the same address for two purposes, and only the client knows which it asked for:
// a continuation is appended, a reload replaces (§8.3, §16.3).
func tasksPage(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := principalOf(r); err != nil {
			unauthenticated(w)
			return
		}

		tasks, err := allTasks(r, store)
		if err != nil {
			fail(w, err)
			return
		}

		filter := filterOf(r)
		page, last, more := filter.page(tasks)

		items := make([]render.Component, 0, len(page))
		for _, task := range page {
			items = append(items, render.TaskRow(task))
		}

		next := ""
		if more {
			next = pageURL(filter, last)
		}
		respond(w, r, render.Page(items, next))
	}
}

func pageURL(filter tasksFilter, after int64) string {
	values := filter.query()
	values.Set("after", strconv.FormatInt(after, 10))
	return tasksPagePath + "?" + values.Encode()
}

const tasksPagePath = "/pages/tasks"

func allTasks(r *http.Request, store domain.Store) ([]domain.Task, error) {
	boards, err := store.Boards(r.Context())
	if err != nil {
		return nil, err
	}
	var tasks []domain.Task
	for _, board := range boards {
		found, err := store.Tasks(r.Context(), board.ID)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, found...)
	}
	return tasks, nil
}
