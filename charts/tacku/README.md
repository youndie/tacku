# The chart

One pod, one volume, one hostname. The parts worth knowing before changing anything:

**It refuses to render rather than deploying something that starts and misbehaves.** Four values
have no default — the hostname, the session key, the issuer and its JWKS address — and each is
refused with what its absence would actually do. All four fail quietly at runtime: an empty
hostname matches nothing, a missing session key makes the server mint a new one at every start and
sign everybody out, and missing issuer settings stop the server from starting at all, which arrives
as a deploy that reports success.

**The JWKS address is not derivable from the issuer.** It is asked of the identity provider's own
discovery document, never guessed from the shape of the issuer URL — providers differ, and the two
that look alike are exactly the ones a guess gets wrong. A wrong address verifies no token at all,
and the failure arrives as every agent being unauthorised rather than as anything about a URL.

**One replica and `Recreate`, because the store is a file.** SQLite on a ReadWriteOnce volume
cannot be served by two pods; a rolling update waits for a volume the outgoing pod still holds and
reads as a deploy that hangs.

**The image tag must be immutable.** Rebuilding under one number leaves helm nothing to notice —
the rendered spec does not change, so nothing restarts — and the deploy is green while the previous
binary keeps running.

**Nothing stands in front of it.** The client is a desktop application and the agent surface is
MCP; a proxy expecting a browser session would reject both. The server checks every route itself,
and four addresses answer without a session because they must: the sign-in form, its submit,
`/healthz`, and the protected-resource metadata RFC 9728 requires to be public.
