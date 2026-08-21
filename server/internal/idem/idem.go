// Package idem runs an operation at most once per idempotency key.
//
// One mechanism serves both surfaces, which is the whole point: the HTTP submit carries the key in
// a header and an MCP tool takes it as an argument, and they must not disagree about what a repeat
// means.
package idem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/youndie/tacku/server/internal/domain"
)

// Once runs fn unless this key has already produced an outcome.
//
// The rule of SPEC.md §16.5, in both directions: the same key with the same request replays the
// first outcome and performs nothing; the same key with a different request is a conflict.
//
// Requests are compared by a hash of their canonical JSON rather than byte for byte. A client is
// free to reorder the keys of an object when it retries, and byte comparison would answer a
// conflict to an honest retry — see Q-01, where the specification leaves this undefined.
// Once is the agent-facing half of the same mechanism: an MCP tool takes the key as an argument,
// there being no header to carry it.
//
// The HTTP half is Middleware, which wraps the handler rather than the operation — see the reason
// there. A tool has no side effects outside its own body, so wrapping the call is enough here.
func Once[T any](ctx context.Context, attempts domain.Attempts, key string, request any, fn func() (T, error)) (T, error) {
	var zero T

	if key == "" {
		return zero, fmt.Errorf("%w: an idempotency key is required for anything that changes state", domain.ErrInvalidTask)
	}

	hash, err := hashRequest(request)
	if err != nil {
		return zero, err
	}

	// Scoped away from the HTTP key space: the same string arriving at both surfaces would
	// otherwise replay one shape of answer into the other.
	scoped := "mcp:" + key

	previous, found, err := attempts.Outcome(ctx, scoped)
	if err != nil {
		return zero, err
	}
	if found {
		if previous.RequestHash != hash {
			return zero, fmt.Errorf("%w: idempotency key %q was used for a different request", domain.ErrConflict, key)
		}
		var replayed T
		if err := json.Unmarshal(previous.Body, &replayed); err != nil {
			return zero, err
		}
		return replayed, nil
	}

	result, err := fn()
	if err != nil {
		// A failed attempt is deliberately not recorded. The rule for the caller is that a key
		// belongs to one attempt and is regenerated after each, whatever the outcome; recording
		// failures would additionally mean a request corrected after a refusal could never succeed
		// under the key it was refused with.
		return zero, err
	}

	body, err := json.Marshal(result)
	if err != nil {
		return zero, err
	}
	if err := attempts.Remember(ctx, scoped, domain.Outcome{RequestHash: hash, Body: body}); err != nil {
		return zero, err
	}
	return result, nil
}

// hashRequest canonicalises through encoding/json, whose object keys are sorted by the marshaller,
// so two requests differing only in key order hash the same.
func hashRequest(request any) (string, error) {
	raw, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	var canonical any
	if err := json.Unmarshal(raw, &canonical); err != nil {
		return "", err
	}
	stable, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(stable)
	return hex.EncodeToString(sum[:]), nil
}
