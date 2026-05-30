-- Schema used by the integration tests. The data definition language is kept in
-- this file rather than in Go string constants so that the generator and the
-- tests apply the same authoritative schema.
--
-- The model is deliberately varied so the tests exercise UUID keys, JSONB
-- documents, NUMERIC and BOOLEAN columns, nullable foreign keys, timestamps
-- with time zone, and three related tables.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

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
