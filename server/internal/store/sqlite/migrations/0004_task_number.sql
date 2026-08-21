-- The number behind a task identifier, in a column of its own.
--
-- Ordering by `id` looked right and was not: TAC-10 sorts before TAC-2, because an identifier that
-- reads like a number is a string to a database. A walk paged by "everything after this one" then
-- returned to tasks it had already handed out, and the failure appeared as a count — twelve tasks
-- seen twenty-four times — rather than as anything resembling a sorting bug.
--
-- Kept as a column rather than derived with substr and a cast, so that the layout of the identifier
-- stays a property of the domain rather than a fact the SQL has to know.
alter table tasks add column number integer not null default 0;

update tasks set number = cast(substr(id, 5) as integer) where number = 0;

create index tasks_by_number on tasks (number);
