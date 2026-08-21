-- One attempt, one outcome.
--
-- SPEC.md §16.5 requires this of a state-changing submit, and the agent surface takes the same key
-- as an ordinary argument rather than growing a second mechanism: an agent retries more often than
-- a person does — on a timeout, on a restart, on the model's own second thoughts.
--
-- The request hash is stored beside the outcome so that the same key carrying a different request
-- is a conflict rather than a silent replay of somebody else's answer.
create table idempotency (
    key          text primary key,
    request_hash text not null,
    body         blob not null,
    created_at   text not null
) strict;
