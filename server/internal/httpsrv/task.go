package httpsrv

import (
	"net/http"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/forms"
	"github.com/youndie/tacku/server/internal/render"
)

// taskFormPrefix names the form of one task.
//
// A form identifier is a path here (see the note beside BulkFormID), and this form exists once per
// task, so the task is part of what it is called: "task-view/TAC-4" submits to
// /submit/task-view/TAC-4. The subject used to travel in `?task=`, which `submit_form` has no way
// to set — so the button posted to an address with no task in it, or to no address at all.
const taskFormPrefix = "task-view/"

// TaskPathPrefix is what a client puts a task identifier after. The graph cannot carry this screen —
// its endpoint is a literal path and there are no parameters — so the client builds the address, and
// the shape of it is part of the contract between the two halves of this repository.
const TaskPathPrefix = "/forms/task/"

// taskScreen shows one task.
//
// A form rather than a screen, and by the usual rule: it takes input — a comment and a status — so it
// carries a schema. Which also means no conditional delivery here, and that is the right trade for
// the screen most likely to be stale.
func taskScreen(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		id := domain.TaskID(r.PathValue("task"))
		task, err := store.Task(r.Context(), id)
		if err != nil {
			fail(w, err)
			return
		}

		history, err := store.TaskChanges(r.Context(), id)
		if err != nil {
			fail(w, err)
			return
		}
		comments, err := store.Comments(r.Context(), id)
		if err != nil {
			fail(w, err)
			return
		}

		form := forms.New(taskFormPrefix + string(id))

		// The second of the two texts a person writes in a tracker, and it was a one-line box for
		// the same reason the description did not exist: the vocabulary has no multiline input
		// (B-29). Shorter than the description — a comment is a remark, not a specification — and
		// the number is the server's either way, because the protocol cannot express a height in
		// lines at all (Q-41).
		comment := render.Column("task-comment-block", 8, nil,
			form.MultilineInput("comment", "Comment", "Write a comment…", "", 3, nil),
			render.Row("task-comment-actions", 0, nil,
				render.Text("task-comment-as", "Posted as you", render.TextMeta),
				render.Spacer("task-comment-spacer"),
				render.PrimaryButton("task-comment-post", "Post", render.SubmitForm(form.FormID()),
					render.PaddingXY(10, 18)),
			),
		)

		// The full selector, and the only place it exists. A board card carries a single-step button
		// instead, so moving backwards or into Blocked is reachable from here and from nowhere else.
		status := form.Select("status", "Move to", "Choose a status…", statusOptions(), nil)

		screen := render.Task{
			Task:     task,
			History:  history,
			Comments: comments,
			Person:   principal.Provenance.OnBehalfOf,
		}.Screen(comment, status)

		respond(w, r, form.Build(screen))
	}
}

// submitTaskView carries both of this screen's actions, which the protocol makes one submit.
//
// A form has one identifier and one payload, so "comment" and "move" arrive together and the handler
// decides by what is filled. That is a real awkwardness rather than a design choice — Q-10 in the
// journal — and it is written out here rather than hidden behind two endpoints that could not both
// be named by one submit_form.
func submitTaskView(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		var request submitRequest
		if err := decodeSubmit(r, &request); err != nil {
			http.Error(w, `{"error":"the body is not a form submission"}`, http.StatusBadRequest)
			return
		}

		id := domain.TaskID(r.PathValue("task"))
		if !id.Valid() {
			fail(w, domain.ErrInvalidTask)
			return
		}

		if status := request.chosen("status"); status != "" {
			// The other half of the count B-36 waits on: a move that arrived here was made by
			// somebody who had already opened the task.
			if _, err := store.MoveTask(r.Context(), id, domain.Status(status), principal.Provenance,
				domain.SurfaceTask); err != nil {
				fail(w, err)
				return
			}
		}
		if text := request.text("comment"); text != "" {
			if _, err := store.Comment(r.Context(), id, text, principal.Provenance); err != nil {
				fail(w, err)
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"type": "navigate", "deeplink": render.LinkTask + string(id),
		})
	}
}
