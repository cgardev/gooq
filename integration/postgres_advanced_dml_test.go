package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// postgres_advanced_dml_test.go exercises the advanced data-manipulation forms
// against the real PostgreSQL container: INSERT ... SELECT, the PostgreSQL-only
// UPDATE ... FROM and DELETE ... USING joins, and the BatchExec helper. All tests
// run inside the per-test transaction from library(t) and bind the default
// (PostgreSQL) dialect, so they never call Using.

func TestPostgresInsertSelect(t *testing.T) {
	t.Run("copies matching rows into the tag table", func(t *testing.T) {
		ctx, tx := library(t)

		// Project each in-print book's id and title into the tag table's
		// (book_id, label) columns; the tag id defaults to gen_random_uuid(). The Go
		// and C books are in print, so two tags are created.
		source := gooq.Select2(db.Book.Id, db.Book.Title).
			From(db.Book).
			Where(db.Book.InPrint.EQ(true))

		_, err := gooq.InsertInto(db.Tag).
			Columns(db.Tag.BookId, db.Tag.Label).
			Select(source).
			Execute(ctx, tx)
		noError(t, "insert ... select", err)

		labels, err := gooq.Select1(db.Tag.Label).
			From(db.Tag).
			OrderBy(db.Tag.Label.Asc()).
			Fetch(ctx, tx)
		noError(t, "read copied tags", err)

		equal(t, "copied tag count", len(labels), 2)
		equal(t, "first copied label", labels[0].V1, "The C Programming Language")
		equal(t, "second copied label", labels[1].V1, "The Go Programming Language")
	})
}

func TestPostgresUpdateFrom(t *testing.T) {
	t.Run("updates books selected through a joined author table", func(t *testing.T) {
		ctx, tx := library(t)

		// UPDATE book ... FROM author joins book to author in the WHERE clause and
		// restricts the update to Kernighan's books, setting their subtitle to a
		// literal. The join therefore decides which rows the SET touches. Two seeded
		// books are by Kernighan.
		_, err := gooq.Update(db.Book).
			Set(db.Book.Subtitle.Set(text("By Brian Kernighan"))).
			From(db.Author).
			Where(db.Book.AuthorId.EQField(db.Author.Id)).
			And(db.Author.Id.EQ(authorKernighan)).
			Execute(ctx, tx)
		noError(t, "update ... from", err)

		rows, err := gooq.Select2(db.Book.Id, db.Book.Subtitle).
			From(db.Book).
			Where(db.Book.AuthorId.EQ(authorKernighan)).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "read updated subtitles", err)

		equal(t, "updated row count", len(rows), 2)
		for _, r := range rows {
			equal(t, "subtitle valid", r.V2.Valid, true)
			equal(t, "subtitle value", r.V2.V, "By Brian Kernighan")
		}

		// The Go book is by Donovan and must be untouched.
		goBook, err := gooq.Select1(db.Book.Subtitle).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchSingle(ctx, tx)
		noError(t, "read untouched subtitle", err)
		equal(t, "go subtitle unchanged", goBook.V1.V, "An Idiomatic Guide")
	})
}

func TestPostgresDeleteUsing(t *testing.T) {
	t.Run("deletes reviews using a joined book table", func(t *testing.T) {
		ctx, tx := library(t)

		// DELETE ... USING book removes every review of an out-of-print book,
		// joining review to book in the WHERE clause. Only The Practice of
		// Programming is out of print, and it carries a single review.
		_, err := gooq.DeleteFrom(db.Review).
			UsingTable(db.Book).
			Where(db.Review.BookId.EQField(db.Book.Id)).
			And(db.Book.InPrint.EQ(false)).
			Execute(ctx, tx)
		noError(t, "delete ... using", err)

		remaining, err := gooq.Select1(db.Review.BookId).
			From(db.Review).
			Where(db.Review.BookId.EQ(bookPractice)).
			Fetch(ctx, tx)
		noError(t, "verify deletion", err)
		equal(t, "remaining practice reviews", len(remaining), 0)

		// The in-print books keep all their reviews: two for Go, one for C.
		all, err := gooq.Select1(db.Review.Id).
			From(db.Review).
			Fetch(ctx, tx)
		noError(t, "count remaining reviews", err)
		equal(t, "remaining review count", len(all), 3)
	})
}

func TestPostgresBatchExec(t *testing.T) {
	t.Run("runs several statements in order and returns one result each", func(t *testing.T) {
		ctx, tx := library(t)

		// Insert two tags, update one, delete the other, all in one batch against
		// the per-test transaction.
		const tagKeep = "d0000000-0000-0000-0000-0000000000b1"
		const tagDrop = "d0000000-0000-0000-0000-0000000000b2"

		results, err := gooq.BatchExec(
			ctx,
			tx,
			gooq.InsertInto(db.Tag).
				Columns(db.Tag.Id, db.Tag.BookId, db.Tag.Label).
				Values(tagKeep, bookGo, "draft"),
			gooq.InsertInto(db.Tag).
				Columns(db.Tag.Id, db.Tag.BookId, db.Tag.Label).
				Values(tagDrop, bookGo, "temporary"),
			gooq.Update(db.Tag).
				Set(db.Tag.Label.Set("classic")).
				Where(db.Tag.Id.EQ(tagKeep)),
			gooq.DeleteFrom(db.Tag).
				Where(db.Tag.Id.EQ(tagDrop)),
		)
		noError(t, "batch exec", err)
		equal(t, "result count", len(results), 4)

		labels, err := gooq.Select1(db.Tag.Label).
			From(db.Tag).
			Where(db.Tag.BookId.EQ(bookGo)).
			OrderBy(db.Tag.Label.Asc()).
			Fetch(ctx, tx)
		noError(t, "read batched tags", err)

		equal(t, "remaining tag count", len(labels), 1)
		equal(t, "the updated label", labels[0].V1, "classic")
	})
}
