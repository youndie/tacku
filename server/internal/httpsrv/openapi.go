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

func ref(target string) map[string]any { return map[string]any{"$ref": target} }
