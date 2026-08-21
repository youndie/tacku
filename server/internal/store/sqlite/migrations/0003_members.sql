-- The people of a workspace, and the only thing that can prove it is them.
--
-- A password rather than nothing, and a hash rather than a password. The prototype could have taken
-- a member identifier and believed it — and that is precisely the shape of a back door that lives
-- for years because it was convenient once.
create table members (
    id         text primary key,
    email      text not null unique,
    name       text not null,
    password   text not null,
    created_at text not null
) strict;
