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

Both surfaces work. A Go server serves screens, forms, pages, a navigation graph, a multi-step
scenario, a live update channel and ten MCP tools; a Compose Desktop client renders them.

The conformance walk passes with every declared endpoint checked and every check having found
something to check — see below for why that second half is the part worth stating.

Four items are open, and none of them is waiting on work: two are measurements that need people
using the product, one needs a dangerous operation to exist before there is anything to put behind
a confirmation, and one is deferred on purpose. The order of work, and what each closed item cost,
is in [backlog.md](backlog.md).

Start with [docs/research/research-architecture.md](docs/research/research-architecture.md): it
records what was verified and against which artefact, which decisions were taken and what they cost.

## The second purpose

kompot was carved out of one application and is claimed to be a toolkit that knows nothing about
any particular domain. That claim had exactly one witness — the application it came from. tacku is
a **second, independent implementation of the protocol**, and it is built under a rule:

> The Go server is written against the published contract — `SPEC.md`, the schema files, the
> conformance kit, the public API of the published artefacts. The Kotlin sources of kompot are not
> in this repository and are not read. A question the contract does not answer is written down
> before it is resolved.

The journal of those questions is [docs/research/questions.md](docs/research/questions.md) — 51
entries — and it is the real output of that half of the project. Sixteen of them were taken
upstream; thirteen are closed, and eight changed the wire: an action that acts on one item of a
list, a route that says what stands behind it, an action on a container, a typography token's
colour, path parameters and a recorded update stream for the walk, a place in the profile for a
deployment's own types, and the truth about what an undeclared type costs.

What a green conformance run means here is defined in
[B-05](docs/backlog/B-05-tck-target-counters-gate.md): every check must find at least one target,
because a check with no targets passes in silence. That guard has since caught the kit growing a
tenth check nobody had noticed.

This repository also uses the extension mechanism it asked for: `date_input` and `multiline_input`
are this deployment's own wire types, declared in its profile and refused by its own validator when
undeclared.

## Layout

| Path | What it is |
|---|---|
| `docs/` | the documentation tree — research, the question journal, the backlog, the designer brief |
| `probes/` | one-off programs, each proving one fact the research relies on |
| `server/` | the Go server: both surfaces, the store, the spec reader |
| `client/` | the Compose Desktop client, the deployment's own wire types, the spec generator and the conformance harness |
| `spec/` | the generated schema files and profile of this build, committed so the Go half can read them |

## Checks

```bash
make check
```

The gate: the documentation, both halves of the code, the formatters, and three guards that exist
because the things they check used to be conventions — every question numbered once, every claim of
having reported something upstream carrying its address, and every wire type this server builds
declared in its profile.

```bash
make tck
```

The conformance walk against a throwaway server. Green means every declared endpoint was fetched
and every check found a target; red is a finding to read and classify, not a broken build.

```bash
make probes
```

Re-run the facts the research rests on — a probe that stops building is a changed fact, not a
broken build.

## Conventions

Code, comments, test names, error messages, commit messages and this README are in English.
The rest of the documentation is in Russian.
