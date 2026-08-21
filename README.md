# tacku

**a lightweight team task tracker where an AI agent is a team member, not a button** — the same
domain is served to people as server-driven screens and to agents as MCP tools

> 🪶 one Go process, two surfaces, one JSON Schema dialect

Two things make tacku different from the other small trackers:

- **Agents write.** An agent files tasks, moves them and comments — on behalf of a named person,
  through [MCP](https://modelcontextprotocol.io). Every change records both actors, and the
  interface shows the difference between "Anna closed it" and "Anna's agent closed it".
- **The client renders what the server describes.** Screens are a tree of components on the wire,
  rendered by [kompot](https://github.com/youndie/kompot); a new screen ships without a new client
  release.

## Status

**Research and backlog only — there is no code yet.** Start with
[docs/research/research-architecture.md](docs/research/research-architecture.md): it records what
was verified and against which artefact, which decisions were taken and what they cost. The order
of work is in [backlog.md](backlog.md).

## The second purpose

kompot was extracted from a banking demo and is claimed to be a toolkit that knows nothing about
any particular domain. That claim has exactly one witness so far — the application the toolkit was
carved out of. tacku is a **second, independent implementation of the protocol**, and it is built
under a rule:

> The Go server is written against the published contract — `SPEC.md`, the schema files, the
> conformance kit. The Kotlin sources of kompot are not in this repository and are not read.
> A question the contract does not answer is written down before it is resolved.

The journal of those questions is [docs/research/questions.md](docs/research/questions.md), and it
is the real output of that half of the project. What a green conformance run means here is defined
in [B-05](docs/backlog/B-05-tck-target-counters-gate.md): every check must find at least one
target, because a check with no targets passes in silence.

## Layout

| Path | What it is |
|---|---|
| `docs/` | the documentation tree — research, backlog, the designer brief |
| `probes/` | one-off programs, each proving one fact the research relies on |
| `server/` | the Go server — not created yet ([B-01](docs/backlog/B-01-repo-and-two-builds.md)) |
| `client/` | the Compose Desktop client, the spec generator and the conformance harness — not created yet |

## Checks

```bash
make check
```

Re-run the facts the research rests on — a probe that stops building is a changed fact, not a
broken build:

```bash
make probes
```

## Conventions

Code, comments, test names, error messages, commit messages and this README are in English.
The rest of the documentation is in Russian.
