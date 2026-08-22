-- The half of the boundary that moves without being asked (B-27).
--
-- `cursor` stays what the feed is measured from. `pending` is where the next visit will start: the
-- end of the journal as of the last time this person was seen. They are two columns and not one
-- because the boundary must not move at the moment the feed is rendered — that would empty the
-- screen the person came to read, and would make two identical requests produce two different
-- bodies, which SPEC.md §16.2 forbids for any screen that hopes to answer 304.
--
-- `away` is how long the person had been gone when the current visit began, in seconds, so the
-- headline can say it without inventing a wall-clock time in a timezone the protocol never carries
-- (Q-31).
alter table seen add column pending text not null default 'c0';
alter table seen add column away integer not null default 0;

-- Existing rows had pressed "mark all as seen" and nothing else. Leaving their `pending` at the
-- default would rewind the boundary to the start of the journal on their next arrival and hand
-- them the entire history of the workspace as news — the exact failure the boundary exists to
-- prevent, arriving through the migration that introduced it.
update seen set pending = cursor;
