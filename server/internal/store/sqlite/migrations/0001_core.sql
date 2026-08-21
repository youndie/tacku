-- The core: boards, tasks, comments and the journal every change writes to.
--
-- The journal is not an audit table added later. Three readers need it — an agent asking what moved
-- since its cursor, a person catching up, and the history on a task — and it is written in the same
-- transaction as the change itself, so a state cannot move without the journal knowing who moved it.

create table boards (
    id    text primary key,
    title text not null
) strict;

-- Task numbers come from here rather than from rowid: the identifier people say out loud must not
-- be whatever the storage happened to assign, or changing the storage renames every task.
create table counters (
    name  text primary key,
    value integer not null
) strict;

create table tasks (
    id         text primary key,
    board      text not null references boards (id),
    title      text not null,
    body       text not null default '',
    status     text not null,
    assignee   text not null default '',
    due        text not null default '',
    created_at text not null,
    updated_at text not null
) strict;

create index tasks_by_board on tasks (board, status);

create table comments (
    id            integer primary key autoincrement,
    task          text not null references tasks (id),
    body          text not null,
    actor_kind    text not null,
    actor_member  text not null,
    actor_version text not null default '',
    on_behalf_of  text not null,
    created_at    text not null
) strict;

create index comments_by_task on comments (task, id);

-- seq is the cursor's coordinate. autoincrement rather than plain rowid on purpose: a plain rowid
-- can be reused after a delete, and a cursor that walks backwards silently replays entries a
-- reader has already seen.
create table changes (
    seq           integer primary key autoincrement,
    task          text not null,
    board         text not null,
    kind          text not null,
    from_value    text not null default '',
    to_value      text not null default '',
    actor_kind    text not null,
    actor_member  text not null,
    actor_version text not null default '',
    on_behalf_of  text not null,
    created_at    text not null
) strict;

create index changes_by_task on changes (task, seq);
