package domain

import (
	"errors"
	"fmt"
)

// ActorKind separates a person from a program acting for one.
type ActorKind string

const (
	ActorHuman ActorKind = "human"
	ActorAgent ActorKind = "agent"
)

// Actor is who performed a change.
//
// Version is carried for agents and empty for people. It is not decoration: when an agent starts
// getting a rule wrong, the first question is which build did it, and a history that cannot answer
// that is a history nobody trusts twice.
type Actor struct {
	Kind    ActorKind
	Member  MemberID
	Version string
}

// Provenance is the pair the whole product rests on: who acted, and on whose behalf.
//
// A team that cannot tell "Anna closed it" from "Anna's agent closed it" stops trusting the history
// as a whole rather than one entry of it — every line then has to be checked by hand, which is more
// expensive than not reading it at all.
type Provenance struct {
	Executor   Actor
	OnBehalfOf MemberID
}

// ErrInvalidProvenance is returned when the pair does not describe anybody real.
var ErrInvalidProvenance = errors.New("domain: invalid provenance")

// Human builds the provenance of a person acting for themselves.
func Human(member MemberID) Provenance {
	return Provenance{
		Executor:   Actor{Kind: ActorHuman, Member: member},
		OnBehalfOf: member,
	}
}

// Agent builds the provenance of an agent acting for a person.
func Agent(agent MemberID, version string, onBehalfOf MemberID) Provenance {
	return Provenance{
		Executor:   Actor{Kind: ActorAgent, Member: agent, Version: version},
		OnBehalfOf: onBehalfOf,
	}
}

// ByAgent reports whether a program performed the change. The interface shows this differently in
// three places, so it is a question the domain has to answer rather than one the renderer guesses.
func (p Provenance) ByAgent() bool { return p.Executor.Kind == ActorAgent }

// Validate rejects the pairs that would make the history unreadable.
//
// The rule that matters is the second one: an agent always acts for somebody. An agent recorded
// with no principal is exactly the entry a reader cannot act on — they can see that something
// automatic happened and not who wanted it.
func (p Provenance) Validate() error {
	if p.Executor.Member == "" {
		return fmt.Errorf("%w: no executor", ErrInvalidProvenance)
	}
	if p.OnBehalfOf == "" {
		return fmt.Errorf("%w: no principal", ErrInvalidProvenance)
	}

	switch p.Executor.Kind {
	case ActorHuman:
		if p.Executor.Member != p.OnBehalfOf {
			return fmt.Errorf("%w: a person acts for themselves, not for %q", ErrInvalidProvenance, p.OnBehalfOf)
		}
		if p.Executor.Version != "" {
			return fmt.Errorf("%w: a person has no version", ErrInvalidProvenance)
		}
	case ActorAgent:
		if p.Executor.Version == "" {
			return fmt.Errorf("%w: an agent must name its version", ErrInvalidProvenance)
		}
	default:
		return fmt.Errorf("%w: unknown actor kind %q", ErrInvalidProvenance, p.Executor.Kind)
	}

	return nil
}
