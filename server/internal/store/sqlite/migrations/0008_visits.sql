-- Every arrival that began a visit, kept rather than overwritten.
--
-- `seen` holds one row per person and answers "where is this reader now". That is the right shape
-- for serving a screen and the wrong one for B-38, which needs the SHAPE of the distribution of
-- gaps: a threshold is only meaningful if there is a trough between "a break inside a day" and
-- "came back tomorrow", and one value per person cannot show a trough.
--
-- Appended rather than counted, because the question is asked once and answered from history. A row
-- per arrival costs a handful of bytes and cannot be reconstructed afterwards from anything at all.
create table visits (
    id     integer primary key autoincrement,
    member text    not null,
    at     text    not null,
    away   integer not null
);

create index visits_by_member on visits (member, id);
