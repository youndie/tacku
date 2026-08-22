package httpsrv

import (
	"encoding/json"
)

// OpenAPI describes this server's HTTP layer.
//
// 3.1 rather than 3.0 because its data model is exactly JSON Schema 2020-12, so a $ref into a
// generated schema file resolves without conversion.
//
// The description is generated from the same file that mounts the routes, and that is the point:
// hand-written, it would describe the server as it was when somebody last remembered to edit it.
// x-kompot-endpoint-kind is what a conformance run reads to know which serialiser a response should
// satisfy — a client picks by kind, never by path.
func OpenAPI(resource string) json.RawMessage {
	document := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "tacku",
			"version": "0.1.0",
		},
		"servers": []any{map[string]any{"url": resource}},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearer": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
			},
		},
		"security": []any{map[string]any{"bearer": []any{}}},
		"paths": map[string]any{
			"/screens/catch-up": map[string]any{
				"get": operation("catchUp", kindScreen,
					ref("kompot-core.schema.json#/$defs/KompotComponent"),
					withNotModified),
			},
			"/pages/changes": map[string]any{
				"get": operation("changesPage", kindPage,
					ref("kompot-standard.schema.json#/$defs/KompotPageResponse")),
			},
			"/screens/board": map[string]any{
				"get": operation("board", kindScreen,
					ref("kompot-core.schema.json#/$defs/KompotComponent"),
					withNotModified),
			},
			"/forms/new-task": map[string]any{
				"get": operation("newTaskForm", kindForm,
					ref("kompot-forms.schema.json#/$defs/KompotFormResponse")),
			},
			"/submit/new-task": map[string]any{
				"post": submitOperation("submitNewTask"),
			},
			"/submit/move": map[string]any{
				"post": submitOperation("submitMove"),
			},
			bulkFormPath: map[string]any{
				"get": operation("bulkMoveForm", kindForm,
					ref("kompot-forms.schema.json#/$defs/KompotFormResponse")),
			},
			bulkSubmitPath: map[string]any{
				"post": submitOperation("submitBulkMove"),
			},
			"/forms/my-tasks": map[string]any{
				"get": operation("myTasks", kindForm,
					ref("kompot-forms.schema.json#/$defs/KompotFormResponse")),
			},
			tasksPagePath: map[string]any{
				"get": operation("tasksPage", kindPage,
					ref("kompot-standard.schema.json#/$defs/KompotPageResponse")),
			},
			"/forms/task/{task}": map[string]any{
				"get": templated(operation("taskScreen", kindForm,
					ref("kompot-forms.schema.json#/$defs/KompotFormResponse"))),
			},
			"/submit/task-view/{task}": map[string]any{
				"post": templated(submitOperation("submitTaskView")),
			},
			wizardStartPath: map[string]any{
				"get": wizardStartOperation("startNewTaskFlow"),
			},
			wizardResumePath: map[string]any{
				"post": wizardResumeOperation("resumeNewTaskFlow"),
			},
			"/forms/new-board": map[string]any{
				"get": operation("newBoardForm", kindForm,
					ref("kompot-forms.schema.json#/$defs/KompotFormResponse")),
			},
			"/submit/new-board": map[string]any{
				"post": submitOperation("submitNewBoard"),
			},
			seenURL: map[string]any{
				"post": submitOperation("submitSeen"),
			},
			"/forms/sign-in": map[string]any{
				"get": public(operation("signInForm", kindForm,
					ref("kompot-forms.schema.json#/$defs/KompotFormResponse"))),
			},
			LoginPath: map[string]any{
				"post": public(operation("submitSignIn", kindSubmit,
					ref("kompot-core.schema.json#/$defs/KompotAction"))),
			},
			"/graph": map[string]any{
				"get": operation("navigationGraph", kindGraph,
					ref("kompot-navigation.schema.json#/$defs/NavigationGraph")),
			},
		},
	}

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		panic(err)
	}
	return json.RawMessage(append(encoded, '\n'))
}

type operationOption func(map[string]any)

// withNotModified declares the conditional half. Only a screen carries it: SPEC.md §16.2 describes
// caching for screens, and extending it silently to other kinds would change what a conformance
// counter means.
func withNotModified(responses map[string]any) {
	responses["304"] = map[string]any{"description": "the client already holds this body"}
}

func operation(id, kind string, response any, options ...operationOption) map[string]any {
	responses := map[string]any{
		"200": map[string]any{
			"description": "ok",
			"content":     map[string]any{"application/json": map[string]any{"schema": response}},
		},
		// Declared on every route because every route is behind the bearer check. A protected
		// endpoint that does not say so is one a conformance run cannot test for it.
		"401": map[string]any{"description": "no token, or a token issued for another resource"},
	}
	for _, option := range options {
		option(responses)
	}
	return map[string]any{
		"operationId":            id,
		"x-kompot-endpoint-kind": kind,
		// Declared on the operation and not only at the root. A conformance run looks for the
		// requirement where it applies, and a document that states it once globally reads to it as
		// a document with no protected endpoints at all — the check then finds nothing to test and
		// passes in silence.
		"security":  []any{map[string]any{"bearer": []any{}}},
		"responses": responses,
	}
}

// submitOperation declares a state-changing submit.
//
// The 400 is not decoration: §16.5 requires the idempotency key and requires the refusal without it,
// and a conformance run tests exactly that. Declaring the key as a header is what tells it where to
// look.
func submitOperation(id string) map[string]any {
	op := operation(id, kindSubmit, ref("kompot-core.schema.json#/$defs/KompotAction"))
	op["parameters"] = []any{map[string]any{
		"name":     "Idempotency-Key",
		"in":       "header",
		"required": true,
		"schema":   map[string]any{"type": "string"},
	}}
	responses, _ := op["responses"].(map[string]any)
	responses["400"] = map[string]any{"description": "no idempotency key"}
	responses["409"] = map[string]any{"description": "the key was used for a different request"}
	responses["422"] = map[string]any{"description": "refused on its merits"}
	return op
}

// wizardStartOperation declares the endpoint that opens a scenario.
//
// The response header is the point of declaring it at all. §16.7 leaves its name to the application
// and describes it in one direction, so this document is the only place a reader — or a harness —
// can learn what to look for and what to send back (Q-50).
func wizardStartOperation(id string) map[string]any {
	op := operation(id, kindWizardStart, ref("kompot-forms.schema.json#/$defs/KompotFormResponse"))
	responses, _ := op["responses"].(map[string]any)
	success, _ := responses["200"].(map[string]any)
	success["headers"] = map[string]any{
		WizardHeader: map[string]any{
			"description": "the scenario to send back with every transition of this flow",
			"required":    true,
			"schema":      map[string]any{"type": "string"},
		},
	}
	return op
}

// wizardResumeOperation declares the endpoint that carries a scenario one transition further.
//
// It asks for the idempotency key like a submit, which §16.5 does not require of it: the transition
// that finishes a flow performs what a submit performs, and a repeated `next` moves a person past a
// step they never pressed past. The decision and its cost are recorded as Q-51.
func wizardResumeOperation(id string) map[string]any {
	op := operation(id, kindWizardResume, ref("kompot-core.schema.json#/$defs/KompotAction"))
	op["parameters"] = []any{
		map[string]any{
			"name":        WizardHeader,
			"in":          "header",
			"required":    true,
			"description": "the scenario answered by wizard_start",
			"schema":      map[string]any{"type": "string"},
		},
		map[string]any{
			"name":     "Idempotency-Key",
			"in":       "header",
			"required": true,
			"schema":   map[string]any{"type": "string"},
		},
	}
	op["requestBody"] = map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": ref("kompot-wizard.schema.json#/$defs/WizardResumeRequest"),
			},
		},
	}
	responses, _ := op["responses"].(map[string]any)
	responses["400"] = map[string]any{"description": "no idempotency key, or a body that is not a transition"}
	responses["404"] = map[string]any{"description": "this scenario has expired, never existed, or belongs to somebody else"}
	responses["409"] = map[string]any{"description": "the key was used for a different request"}
	responses["422"] = map[string]any{"description": "refused on its merits"}
	return op
}

// public marks the two routes a person with no session must be able to reach.
//
// The empty security list is how OpenAPI says "this one needs nothing", overriding the document's
// default — and it matters beyond documentation: a conformance run reads it to decide which
// endpoints to try anonymously, and a sign-in form declared as protected would be tested for a 401
// it must never give.
//
// The 401 stays on the submit, and that is not a contradiction. §16.8: the absence of authorisation
// and the code 401 are independent, and a sign-in answering a wrong pair is the example it gives.
func public(op map[string]any) map[string]any {
	op["security"] = []any{}
	responses, _ := op["responses"].(map[string]any)
	if op["x-kompot-endpoint-kind"] == kindForm {
		delete(responses, "401")
	}
	return op
}

// templated declares the one path parameter this server has.
//
// A screen addressed by naming a thing cannot be in the navigation graph — a route's endpoint is a
// literal path — so it is resolved by the client from a prefix, and described here so that the
// description still covers every route the server answers.
func templated(op map[string]any) map[string]any {
	op["parameters"] = []any{map[string]any{
		"name":     "task",
		"in":       "path",
		"required": true,
		"schema":   map[string]any{"type": "string", "pattern": "^TAC-[1-9][0-9]*$"},
	}}
	return op
}

func ref(target string) map[string]any { return map[string]any{"$ref": target} }
