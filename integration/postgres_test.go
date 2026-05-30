package integration

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// These tests read top to bottom as small stories. Each opens a seeded library,
// runs one fluent jooq query, and asserts the typed result. The container, the
// schema, the seeding, and the per-test rollback all live in harness_test.go, so
// nothing here repeats the plumbing.

func TestUUID(t *testing.T) {
	t.Run("round-trips a uuid primary key inserted through the builder", func(t *testing.T) {
		ctx, tx := library(t)

		const id = "d0000000-0000-0000-0000-000000000001"
		_, err := gooq.InsertInto(db.Author).
			Columns(db.Author.Id, db.Author.Name, db.Author.Email, db.Author.Metadata).
			Values(id, "Margaret Hamilton", "hamilton@example.com", noDoc()).
			Execute(ctx, tx)
		noError(t, "insert author", err)

		row, err := gooq.Select1(db.Author.Id).
			From(db.Author).
			Where(db.Author.Id.EQ(id)).
			FetchSingle(ctx, tx)
		noError(t, "read back id", err)
		equal(t, "round-tripped id", row.V1, id)
	})

	t.Run("normalizes an uppercase uuid to canonical lowercase on read-back", func(t *testing.T) {
		ctx, tx := library(t)

		const upper = "D0000000-0000-0000-0000-0000000000AB"
		const lower = "d0000000-0000-0000-0000-0000000000ab"
		_, err := gooq.InsertInto(db.Author).
			Columns(db.Author.Id, db.Author.Name, db.Author.Email, db.Author.Metadata).
			Values(upper, "Katherine Johnson", "johnson@example.com", noDoc()).
			Execute(ctx, tx)
		noError(t, "insert with uppercase uuid", err)

		row, err := gooq.Select1(db.Author.Id).
			From(db.Author).
			Where(db.Author.Id.EQ(upper)).
			FetchSingle(ctx, tx)
		noError(t, "read back normalized id", err)
		equal(t, "uuid normalized to lowercase", row.V1, lower)
	})

	t.Run("joins book to author on uuid foreign keys", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select2(db.Book.Title, db.Author.Name).
			From(db.Book).
			InnerJoin(db.Author).On(db.Book.AuthorId.EQField(db.Author.Id)).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "join on uuid foreign key", err)

		equal(t, "count", len(rows), 3)
		equal(t, "first book", rows[0].V1, "The Go Programming Language")
		equal(t, "first author", rows[0].V2, "Alan Donovan")
		equal(t, "third author", rows[2].V2, "Brian Kernighan")
	})

	t.Run("filters with In over several uuid strings", func(t *testing.T) {
		ctx, tx := library(t)

		titles, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.In(bookGo, bookC)).
			OrderBy(db.Book.Title.Asc()).
			Fetch(ctx, tx)
		noError(t, "filter by uuid set", err)

		equal(t, "count", len(titles), 2)
		equal(t, "first title", titles[0].V1, "The C Programming Language")
		equal(t, "second title", titles[1].V1, "The Go Programming Language")
	})

	t.Run("reads a present editor as valid with the right id", func(t *testing.T) {
		ctx, tx := library(t)

		row, err := gooq.Select1(db.Book.EditorId).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchSingle(ctx, tx)
		noError(t, "read editor", err)

		equal(t, "valid", row.V1.Valid, true)
		equal(t, "editor id", row.V1.V, authorKernighan)
	})

	t.Run("reads an absent editor as invalid", func(t *testing.T) {
		ctx, tx := library(t)

		row, err := gooq.Select1(db.Book.EditorId).
			From(db.Book).
			Where(db.Book.Id.EQ(bookPractice)).
			FetchSingle(ctx, tx)
		noError(t, "read editor", err)

		equal(t, "valid", row.V1.Valid, false)
	})

	t.Run("partitions books by whether the editor is null", func(t *testing.T) {
		ctx, tx := library(t)

		withoutEditor, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.EditorId.IsNull()).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "books with no editor", err)
		equal(t, "count without editor", len(withoutEditor), 2)
		equal(t, "first without editor", withoutEditor[0].V1, bookPractice)
		equal(t, "second without editor", withoutEditor[1].V1, bookC)

		withEditor, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.EditorId.IsNotNull()).
			Fetch(ctx, tx)
		noError(t, "books with an editor", err)
		equal(t, "count with editor", len(withEditor), 1)
		equal(t, "the book with an editor", withEditor[0].V1, bookGo)
	})
}

func TestJSONB(t *testing.T) {
	t.Run("round-trips a JSON document with nested fields", func(t *testing.T) {
		ctx, tx := library(t)

		row, err := gooq.Select1(db.Author.Metadata).
			From(db.Author).
			Where(db.Author.Id.EQ(authorDonovan)).
			FetchSingle(ctx, tx)
		noError(t, "fetch metadata", err)

		var metadata struct {
			Country string   `json:"country"`
			Awards  []string `json:"awards"`
		}
		noError(t, "unmarshal metadata", json.Unmarshal(row.V1, &metadata))
		equal(t, "object key", metadata.Country, "US")
		equal(t, "array length", len(metadata.Awards), 1)
		equal(t, "array element", metadata.Awards[0], "Hugo")
	})

	t.Run("scans a NULL metadata as a nil byte slice", func(t *testing.T) {
		ctx, tx := library(t)

		// A nullable JSON column is generated as a gooq.Field[[]byte], so a SQL
		// NULL scans directly into a nil slice. (A json.RawMessage destination
		// would fail here, because database/sql only assigns a nil driver value
		// to *[]byte, *sql.RawBytes, or *any.)
		row, err := gooq.Select1(db.Author.Metadata).
			From(db.Author).
			Where(db.Author.Id.EQ(authorKernighan)).
			FetchSingle(ctx, tx)
		noError(t, "fetch null metadata", err)
		equal(t, "metadata length", len(row.V1), 0)
		if row.V1 != nil {
			t.Errorf("metadata = %v, want nil", row.V1)
		}
	})

	t.Run("filters on metadata IS NULL", func(t *testing.T) {
		ctx, tx := library(t)

		ids, err := gooq.Select1(db.Author.Id).
			From(db.Author).
			Where(db.Author.Metadata.IsNull()).
			Fetch(ctx, tx)
		noError(t, "filter null metadata", err)

		equal(t, "count", len(ids), 1)
		equal(t, "the author without metadata", ids[0].V1, authorKernighan)
	})

	t.Run("upserts a jsonb column to the excluded document", func(t *testing.T) {
		ctx, tx := library(t)

		updated := doc(t, map[string]any{"format": "ebook", "languages": []any{"en", "fr", "ja"}})
		_, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes,
			).
			Values(bookGo, authorDonovan, "The Go Programming Language", 39.99,
				int64(380), true, updated).
			OnConflict(db.Book.Id).
			DoUpdateSet(gooq.SetToExcluded(db.Book.Attributes)).
			Execute(ctx, tx)
		noError(t, "upsert attributes", err)

		row, err := gooq.Select1(db.Book.Attributes).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchSingle(ctx, tx)
		noError(t, "read back attributes", err)

		var attributes struct {
			Format    string   `json:"format"`
			Languages []string `json:"languages"`
		}
		noError(t, "unmarshal attributes", json.Unmarshal(row.V1, &attributes))
		equal(t, "updated format", attributes.Format, "ebook")
		equal(t, "updated language count", len(attributes.Languages), 3)
		equal(t, "third language", attributes.Languages[2], "ja")
	})
}

func TestNumericIntegerBoolean(t *testing.T) {
	t.Run("scans a numeric price to the exact seeded float64", func(t *testing.T) {
		ctx, tx := library(t)

		row, err := gooq.Select1(db.Book.Price).
			From(db.Book).
			Where(db.Book.Id.EQ(bookPractice)).
			FetchSingle(ctx, tx)
		noError(t, "fetch price", err)
		equal(t, "price", row.V1, 29.50)
	})

	t.Run("filters a price range with Between, ordered", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select2(db.Book.Title, db.Book.Price).
			From(db.Book).
			Where(db.Book.Price.Between(29.50, 39.99)).
			OrderBy(db.Book.Price.Asc()).
			Fetch(ctx, tx)
		noError(t, "fetch price range", err)

		equal(t, "count", len(rows), 2)
		equal(t, "cheapest title", rows[0].V1, "The Practice of Programming")
		equal(t, "cheapest price", rows[0].V2, 29.50)
		equal(t, "dearest in range", rows[1].V2, 39.99)
	})

	t.Run("compares integer page counts with GT and LE", func(t *testing.T) {
		ctx, tx := library(t)

		thick, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.PageCount.GT(300)).
			Fetch(ctx, tx)
		noError(t, "fetch thick books", err)
		equal(t, "thick count", len(thick), 1)
		equal(t, "the thick book", thick[0].V1, "The Go Programming Language")

		thin, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.PageCount.LE(272)).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "fetch thin books", err)
		equal(t, "thin count", len(thin), 2)
		equal(t, "first thin", thin[0].V1, bookPractice)
		equal(t, "second thin", thin[1].V1, bookC)
	})

	t.Run("partitions books by the boolean in_print flag", func(t *testing.T) {
		ctx, tx := library(t)

		inPrint, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.InPrint.EQ(true)).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "fetch in-print books", err)
		equal(t, "in-print count", len(inPrint), 2)
		equal(t, "first in print", inPrint[0].V1, bookGo)
		equal(t, "second in print", inPrint[1].V1, bookC)

		outOfPrint, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.InPrint.EQ(false)).
			Fetch(ctx, tx)
		noError(t, "fetch out-of-print books", err)
		equal(t, "out-of-print count", len(outOfPrint), 1)
		equal(t, "the out-of-print book", outOfPrint[0].V1, bookPractice)
	})
}

func TestTimestamps(t *testing.T) {
	t.Run("round-trips an exact UTC instant in a nullable timestamp", func(t *testing.T) {
		ctx, tx := library(t)

		row, err := gooq.Select1(db.Book.PublishedAt).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchSingle(ctx, tx)
		noError(t, "fetch published_at", err)

		equal(t, "valid", row.V1.Valid, true)
		if !row.V1.V.Equal(publishedGo) {
			t.Errorf("published at = %v, want %v", row.V1.V, publishedGo)
		}
	})

	t.Run("orders by the publication timestamp", func(t *testing.T) {
		ctx, tx := library(t)

		// Only one book carries a publication date; restrict to the dated rows so
		// the ordering is meaningful and deterministic.
		rows, err := gooq.Select2(db.Book.Id, db.Book.PublishedAt).
			From(db.Book).
			Where(db.Book.PublishedAt.IsNotNull()).
			OrderBy(db.Book.PublishedAt.Asc()).
			Fetch(ctx, tx)
		noError(t, "order by published_at", err)
		equal(t, "dated count", len(rows), 1)
		equal(t, "the dated book", rows[0].V1, bookGo)
	})

	t.Run("orders by the creation timestamp", func(t *testing.T) {
		ctx, tx := library(t)

		// created_at was defaulted by the database; every row has a value, so the
		// ordering simply must succeed and return every book.
		rows, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			OrderBy(db.Book.CreatedAt.Asc(), db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "order by created_at", err)
		equal(t, "count", len(rows), 3)
	})

	t.Run("filters published_at with IS NULL and IS NOT NULL", func(t *testing.T) {
		ctx, tx := library(t)

		unpublished, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.PublishedAt.IsNull()).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "fetch unpublished", err)
		equal(t, "unpublished count", len(unpublished), 2)

		published, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.PublishedAt.IsNotNull()).
			Fetch(ctx, tx)
		noError(t, "fetch published", err)
		equal(t, "published count", len(published), 1)
		equal(t, "the published book", published[0].V1, bookGo)
	})
}

func TestMultiTable(t *testing.T) {
	t.Run("joins author, book, and review three ways", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select3(db.Review.Reviewer, db.Book.Title, db.Author.Name).
			From(db.Review).
			InnerJoin(db.Book).On(db.Review.BookId.EQField(db.Book.Id)).
			InnerJoin(db.Author).On(db.Book.AuthorId.EQField(db.Author.Id)).
			Where(db.Review.Rating.EQ(5)).
			OrderBy(db.Review.Reviewer.Asc()).
			Fetch(ctx, tx)
		noError(t, "three-way join", err)

		equal(t, "count", len(rows), 2)
		equal(t, "first reviewer", rows[0].V1, "Ada")
		equal(t, "first book", rows[0].V2, "The Go Programming Language")
		equal(t, "first author", rows[0].V3, "Alan Donovan")
		equal(t, "second reviewer", rows[1].V1, "Grace")
		equal(t, "second book", rows[1].V2, "The Practice of Programming")
		equal(t, "second author", rows[1].V3, "Brian Kernighan")
	})

	t.Run("joins an aliased table and keeps the alias in the SQL", func(t *testing.T) {
		ctx, tx := library(t)

		author := db.Author.As("a")
		query := gooq.Select2(db.Book.Title, author.Name).
			From(db.Book).
			InnerJoin(author).On(db.Book.AuthorId.EQField(author.Id)).
			OrderBy(db.Book.Id.Asc())

		rendered, _, err := query.SQL()
		noError(t, "render SQL", err)
		if !strings.Contains(rendered, `"author" "a"`) {
			t.Errorf("expected the alias %q in the SQL, got: %s", `"author" "a"`, rendered)
		}

		rows, err := query.Fetch(ctx, tx)
		noError(t, "fetch aliased join", err)
		equal(t, "count", len(rows), 3)
		equal(t, "first book", rows[0].V1, "The Go Programming Language")
		equal(t, "first author", rows[0].V2, "Alan Donovan")
		equal(t, "third author", rows[2].V2, "Brian Kernighan")
	})
}

func TestPredicates(t *testing.T) {
	t.Run("In with an empty list returns zero rows", func(t *testing.T) {
		ctx, tx := library(t)

		query := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.In())

		rendered, _, err := query.SQL()
		noError(t, "render empty In", err)
		if !strings.Contains(rendered, "1 = 0") {
			t.Errorf("expected the empty-IN guard %q in the SQL, got: %s", "1 = 0", rendered)
		}

		rows, err := query.Fetch(ctx, tx)
		noError(t, "fetch empty In", err)
		equal(t, "count", len(rows), 0)
	})

	t.Run("In with a single value returns exactly that row", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.In(bookC)).
			Fetch(ctx, tx)
		noError(t, "fetch single In", err)
		equal(t, "count", len(rows), 1)
		equal(t, "title", rows[0].V1, "The C Programming Language")
	})

	t.Run("NotIn excludes the listed rows", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.NotIn(bookGo, bookPractice)).
			Fetch(ctx, tx)
		noError(t, "fetch NotIn", err)
		equal(t, "count", len(rows), 1)
		equal(t, "the remaining book", rows[0].V1, bookC)
	})

	t.Run("composes deeply nested And, Or, and Not", func(t *testing.T) {
		ctx, tx := library(t)

		// (in print AND price < 40) OR NOT(page_count > 270), evaluated per book:
		//   Go:       (true AND true)  OR NOT(true)  = true
		//   Practice: (false AND true) OR NOT(false) = true
		//   C:        (true AND false) OR NOT(true)  = false
		condition := db.Book.InPrint.EQ(true).And(db.Book.Price.LT(40)).
			Or(db.Book.PageCount.GT(270).Not())

		rows, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(condition).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "fetch nested predicate", err)
		equal(t, "count", len(rows), 2)
		equal(t, "first match", rows[0].V1, bookGo)
		equal(t, "second match", rows[1].V1, bookPractice)
	})

	t.Run("reuses a stored Condition variable across two queries", func(t *testing.T) {
		ctx, tx := library(t)

		inPrint := db.Book.InPrint.EQ(true)

		ids, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(inPrint).
			Fetch(ctx, tx)
		noError(t, "first reuse", err)
		equal(t, "in-print count", len(ids), 2)

		titles, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(inPrint).
			And(db.Book.Price.GT(40)).
			Fetch(ctx, tx)
		noError(t, "second reuse", err)
		equal(t, "expensive in-print count", len(titles), 1)
		equal(t, "the expensive in-print book", titles[0].V1, "The C Programming Language")
	})

	t.Run("matches with Like wildcards, NotLike, and ILike", func(t *testing.T) {
		ctx, tx := library(t)

		like, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Title.Like("The Go%")).
			Fetch(ctx, tx)
		noError(t, "Like percent", err)
		equal(t, "Like count", len(like), 1)
		equal(t, "Like title", like[0].V1, "The Go Programming Language")

		// "The _ Programming Language" matches a single character where the
		// underscore stands, which is only the C book.
		underscore, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Title.Like("The _ Programming Language")).
			Fetch(ctx, tx)
		noError(t, "Like underscore", err)
		equal(t, "underscore count", len(underscore), 1)
		equal(t, "underscore title", underscore[0].V1, "The C Programming Language")

		notLike, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Title.NotLike("The Go%")).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "NotLike", err)
		equal(t, "NotLike count", len(notLike), 2)

		ilike, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Title.ILike("the go%")).
			Fetch(ctx, tx)
		noError(t, "ILike", err)
		equal(t, "ILike count", len(ilike), 1)
		equal(t, "ILike title", ilike[0].V1, "The Go Programming Language")
	})

	t.Run("binds a title with a quote and unicode, proving values are bound", func(t *testing.T) {
		ctx, tx := library(t)

		const tricky = "O'Reilly — Go ☕"
		_, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes,
			).
			Values("e0000000-0000-0000-0000-000000000001", authorDonovan, tricky, 12.34,
				int64(100), true, doc(t, map[string]any{"format": "zine"})).
			Execute(ctx, tx)
		noError(t, "insert tricky title", err)

		row, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Title.EQ(tricky)).
			FetchSingle(ctx, tx)
		noError(t, "read back tricky title", err)
		equal(t, "tricky title round-trips", row.V1, tricky)
	})
}

func TestFetchCardinality(t *testing.T) {
	t.Run("FetchOne returns the zero value when nothing matches", func(t *testing.T) {
		ctx, tx := library(t)

		row, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.EQ("00000000-0000-0000-0000-000000000000")).
			FetchOne(ctx, tx)
		noError(t, "fetch one", err)
		equal(t, "zero title", row.V1, "")
	})

	t.Run("FetchSingle reports no rows", func(t *testing.T) {
		ctx, tx := library(t)

		_, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.EQ("00000000-0000-0000-0000-000000000000")).
			FetchSingle(ctx, tx)
		isError(t, "FetchSingle(no rows)", err, sql.ErrNoRows)
	})

	t.Run("FetchSingle reports too many rows", func(t *testing.T) {
		ctx, tx := library(t)

		_, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			FetchSingle(ctx, tx)
		isError(t, "FetchSingle(many rows)", err, gooq.ErrTooManyRows)
	})
}

func TestInsert(t *testing.T) {
	t.Run("inserts an explicit id and returns it", func(t *testing.T) {
		ctx, tx := library(t)

		const id = "f0000000-0000-0000-0000-000000000001"
		query, args, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes,
			).
			Values(id, authorKernighan, "Unix: A History and a Memoir", 19.99,
				int64(192), true, doc(t, map[string]any{"format": "paperback"})).
			Returning(db.Book.Id).
			SQL()
		noError(t, "render INSERT ... RETURNING", err)

		var returned string
		noError(t, "execute insert", tx.QueryRowContext(ctx, query, args...).Scan(&returned))
		equal(t, "returned id", returned, id)
	})

	t.Run("omits the id so the database default generates one", func(t *testing.T) {
		ctx, tx := library(t)

		query, args, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes,
			).
			Values(authorKernighan, "Software Tools", 24.99,
				int64(338), false, doc(t, map[string]any{"format": "paperback"})).
			Returning(db.Book.Id).
			SQL()
		noError(t, "render default-id INSERT ... RETURNING", err)

		var generated string
		noError(t, "execute default-id insert", tx.QueryRowContext(ctx, query, args...).Scan(&generated))
		equal(t, "generated uuid length", len(generated), 36)
	})
}

func TestUpdate(t *testing.T) {
	t.Run("updates a row and reports it was affected", func(t *testing.T) {
		ctx, tx := library(t)

		result, err := gooq.Update(db.Book).
			Set(db.Book.Price.Set(9.99)).
			Where(db.Book.Id.EQ(bookPractice)).
			Execute(ctx, tx)
		noError(t, "update price", err)

		affected, err := result.RowsAffected()
		noError(t, "rows affected", err)
		equal(t, "rows affected", affected, 1)

		row, err := gooq.Select1(db.Book.Price).
			From(db.Book).
			Where(db.Book.Id.EQ(bookPractice)).
			FetchSingle(ctx, tx)
		noError(t, "read back price", err)
		equal(t, "updated price", row.V1, 9.99)
	})
}

func TestDelete(t *testing.T) {
	t.Run("deletes a row and returns its id", func(t *testing.T) {
		ctx, tx := library(t)

		// The C book has one review that references it; remove the review first so
		// the foreign key constraint permits deleting the book.
		_, err := gooq.DeleteFrom(db.Review).
			Where(db.Review.BookId.EQ(bookC)).
			Execute(ctx, tx)
		noError(t, "delete dependent reviews", err)

		query, args, err := gooq.DeleteFrom(db.Book).
			Where(db.Book.Id.EQ(bookC)).
			Returning(db.Book.Id).
			SQL()
		noError(t, "render DELETE ... RETURNING", err)

		var returned string
		noError(t, "execute delete", tx.QueryRowContext(ctx, query, args...).Scan(&returned))
		equal(t, "returned id", returned, bookC)

		remaining, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.EQ(bookC)).
			Fetch(ctx, tx)
		noError(t, "verify deletion", err)
		equal(t, "remaining", len(remaining), 0)
	})
}

func TestUpsert(t *testing.T) {
	t.Run("ON CONFLICT DO UPDATE overwrites the conflicting row", func(t *testing.T) {
		ctx, tx := library(t)

		_, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes,
			).
			Values(bookGo, authorDonovan, "The Go Programming Language, 2nd Edition", 49.99,
				int64(400), true, doc(t, map[string]any{"format": "hardcover"})).
			OnConflict(db.Book.Id).
			DoUpdateSet(gooq.SetToExcluded(db.Book.Title), gooq.SetToExcluded(db.Book.Price)).
			Execute(ctx, tx)
		noError(t, "upsert do update", err)

		row, err := gooq.Select2(db.Book.Title, db.Book.Price).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchSingle(ctx, tx)
		noError(t, "read back upserted row", err)
		equal(t, "updated title", row.V1, "The Go Programming Language, 2nd Edition")
		equal(t, "updated price", row.V2, 49.99)
	})

	t.Run("ON CONFLICT DO NOTHING leaves the row untouched", func(t *testing.T) {
		ctx, tx := library(t)

		_, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes,
			).
			Values(bookGo, authorDonovan, "Should Not Replace", 0.0,
				int64(1), false, doc(t, map[string]any{"format": "none"})).
			OnConflict(db.Book.Id).
			DoNothing().
			Execute(ctx, tx)
		noError(t, "upsert do nothing", err)

		row, err := gooq.Select2(db.Book.Title, db.Book.Price).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchSingle(ctx, tx)
		noError(t, "read back untouched row", err)
		equal(t, "title unchanged", row.V1, "The Go Programming Language")
		equal(t, "price unchanged", row.V2, 39.99)
	})
}
