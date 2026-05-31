-- Schema used by the integration tests. The data definition language is kept in
-- this file rather than in Go string constants so that the generator and the
-- tests apply the same authoritative schema.
--
-- The model is deliberately varied so the tests exercise UUID keys, JSONB
-- documents, NUMERIC and BOOLEAN columns, nullable foreign keys, timestamps
-- with time zone, an enumerated type, a multi-column unique constraint, and a
-- view, across several related tables.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- book_status is an enumerated type exercised by the code generator's enum
-- support. The generator emits a Go named string type with one constant per
-- label, in this declared order.
CREATE TYPE book_status AS ENUM ('draft', 'in_print', 'out_of_print');

CREATE TABLE author (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    metadata   JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE book (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id    UUID NOT NULL REFERENCES author (id),
    editor_id    UUID REFERENCES author (id),
    title        TEXT NOT NULL,
    subtitle     TEXT,
    price        NUMERIC(8, 2) NOT NULL,
    page_count   INTEGER NOT NULL,
    in_print     BOOLEAN NOT NULL,
    status       book_status NOT NULL DEFAULT 'in_print',
    attributes   JSONB NOT NULL,
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE review (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    book_id   UUID NOT NULL REFERENCES book (id),
    reviewer  TEXT NOT NULL,
    rating    INTEGER NOT NULL,
    body      TEXT,
    posted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- tag exercises the new key metadata emission: a single-column primary key, a
-- foreign key to book, and a multi-column unique constraint over (book_id,
-- label) so a book cannot carry the same label twice.
CREATE TABLE tag (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    book_id UUID NOT NULL REFERENCES book (id),
    label   TEXT NOT NULL,
    CONSTRAINT tag_book_id_label_key UNIQUE (book_id, label)
);

-- book_overview is a view exercising view code generation: the generator emits
-- column accessors only, with no key metadata.
CREATE VIEW book_overview AS
SELECT
    b.id          AS book_id,
    b.title       AS title,
    b.status      AS status,
    a.name        AS author_name
FROM book b
JOIN author a ON a.id = b.author_id;
