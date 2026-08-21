-- Where a person had read up to.
--
-- Only the explicit half of the boundary: pressing "mark all as seen" moves it. Whether it should
-- also move on its own — after an hour away, on closing the window — is the open question B-27, and
-- guessing an answer here would bury it in a table.
create table seen (
    member text primary key,
    cursor text not null,
    at     text not null
) strict;
