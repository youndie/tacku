package mcpsrv

// Inputs of the tools. Descriptions carry into the generated inputSchema, so they are read by the
// model at the moment it decides what to send.

type boardView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type boardsOut struct {
	Boards []boardView `json:"boards"`
}

type listTasksIn struct {
	Board  string `json:"board" jsonschema:"the board identifier from list_boards"`
	Status string `json:"status,omitempty" jsonschema:"optional filter: todo, in_progress, in_review, done or blocked"`
}

type tasksOut struct {
	Tasks []taskBrief `json:"tasks"`
}

type taskRef struct {
	Task string `json:"task" jsonschema:"a task identifier such as TAC-124"`
}

type changesIn struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"the cursor from your previous call; empty means from the beginning"`
	Limit  int    `json:"limit,omitempty" jsonschema:"how many entries at most; defaults to 50"`
}

type changesOut struct {
	Changes []changeView `json:"changes"`
	Cursor  string       `json:"cursor" jsonschema:"pass this back next time; it advances only over what you were given"`
}

// The write inputs all carry an idempotency key. It is required rather than optional because an
// agent retries more often than a person: on a timeout, on a restart, on the model's own second
// thoughts. One key belongs to one attempt — generate a new one when you deliberately change the
// request, and reuse the same one when you are repeating a call whose outcome you did not see.

type createTaskIn struct {
	Board          string `json:"board"`
	Title          string `json:"title"`
	Body           string `json:"body,omitempty"`
	Assignee       string `json:"assignee,omitempty"`
	Due            string `json:"due,omitempty" jsonschema:"YYYY-MM-DD"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"a key for this attempt; repeating a call with the same key does not create a second task"`
}

type moveTaskIn struct {
	Task           string `json:"task"`
	Status         string `json:"status" jsonschema:"todo, in_progress, in_review, done or blocked"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"a key for this attempt"`
}

type assignTaskIn struct {
	Task           string `json:"task"`
	Assignee       string `json:"assignee" jsonschema:"a member identifier; empty unassigns"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"a key for this attempt"`
}

type setDueIn struct {
	Task           string `json:"task"`
	Due            string `json:"due" jsonschema:"YYYY-MM-DD; empty clears the date"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"a key for this attempt"`
}

type commentIn struct {
	Task           string `json:"task"`
	Text           string `json:"text"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"a key for this attempt"`
}

type taskOut struct {
	Task taskBrief `json:"task"`
}
