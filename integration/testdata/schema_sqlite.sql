-- SQLite schema used by the pure-Go SQLite integration tests. It mirrors the
-- authoritative PostgreSQL schema in schema.sql table for table and column for
-- column, so the same generated accessors (db.Author, db.Book, db.Review) work
-- unchanged. Only the column TYPES differ, mapped to SQLite's storage classes:
--
--   UUID         -> TEXT      (identifiers are inserted explicitly by the tests)
--   JSONB / JSON -> TEXT      (documents are stored and read back as raw bytes)
--   BIGINT / INT -> INTEGER
--   NUMERIC / DOUBLE PRECISION -> REAL
--   BOOLEAN      -> INTEGER   (0 / 1)
--   TIMESTAMPTZ  -> TIMESTAMP (declared so the modernc driver round-trips the
--                              stored text back into a Go time.Time on scan; the
--                              column still has TEXT affinity in SQLite)
--
-- SQLite has no gen_random_uuid() or now() defaults, so the columns are defined
-- plainly and every test supplies explicit identifiers and timestamps.

CREATE TABLE author (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    metadata   TEXT,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE book (
    id           TEXT PRIMARY KEY,
    author_id    TEXT NOT NULL REFERENCES author (id),
    editor_id    TEXT REFERENCES author (id),
    title        TEXT NOT NULL,
    subtitle     TEXT,
    price        REAL NOT NULL,
    page_count   INTEGER NOT NULL,
    in_print     INTEGER NOT NULL,
    attributes   TEXT NOT NULL,
    published_at TIMESTAMP,
    created_at   TIMESTAMP NOT NULL
);

CREATE TABLE review (
    id        TEXT PRIMARY KEY,
    book_id   TEXT NOT NULL REFERENCES book (id),
    reviewer  TEXT NOT NULL,
    rating    INTEGER NOT NULL,
    body      TEXT,
    posted_at TIMESTAMP NOT NULL
);
