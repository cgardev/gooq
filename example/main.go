// Command example demonstrates the jooq-for-go query builder against the
// generated table accessors in ./internal/db. It renders representative queries
// rather than connecting to a database, so it runs with no external dependency
// and showcases the single-AST, render-per-dialect design.
package main

import (
	"fmt"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/example/internal/db"
)

type sqlNode interface {
	SQL() (string, []any, error)
}

type sqlForNode interface {
	SQLFor(gooq.Dialect) (string, []any, error)
}

func show(label string, q sqlNode) {
	sql, args, err := q.SQL()
	if err != nil {
		fmt.Printf("  %-22s ERROR: %v\n", label, err)
		return
	}
	fmt.Printf("  %-22s %s\n", label, sql)
	if len(args) > 0 {
		fmt.Printf("  %-22s args: %v\n", "", args)
	}
}

func showFor(label string, q sqlForNode, d gooq.Dialect) {
	sql, _, err := q.SQLFor(d)
	if err != nil {
		fmt.Printf("  %-22s ERROR: %v\n", label, err)
		return
	}
	fmt.Printf("  %-22s %s\n", label, sql)
}

func main() {
	fmt.Println("Typed SELECT with WHERE, ORDER BY, LIMIT:")
	// db.Book.Price is a NumericField[float64]; GT requires a float64, so a
	// mismatched type would be a compile error, not a runtime surprise.
	q := gooq.Select2(db.Book.Title, db.Book.Price).
		From(db.Book).
		Where(db.Book.Price.GT(10)).
		OrderBy(db.Book.Title.Asc()).
		Limit(20)
	show("postgres", q)

	fmt.Println("\nThe same query AST rendered for three dialects:")
	showFor("postgres", q, gooq.Postgres())
	showFor("mysql", q, gooq.MySQL())
	showFor("sqlite", q, gooq.SQLite())

	fmt.Println("\nTyped JOIN with an aliased table:")
	a := db.Author.As("a")
	join := gooq.Select2(db.Book.Title, a.Name).
		From(db.Book).
		LeftJoin(a).On(db.Book.AuthorId.EQField(a.Id))
	show("join", join)

	fmt.Println("\nINSERT with upsert (ON CONFLICT DO UPDATE):")
	ins := gooq.InsertInto(db.Book).
		Columns(db.Book.Id, db.Book.Title, db.Book.Price).
		Values(int64(1), "The Go Programming Language", 39.99).
		OnConflict(db.Book.Id).
		DoUpdateSet(gooq.SetToExcluded(db.Book.Title), gooq.SetToExcluded(db.Book.Price))
	show("postgres", ins)
	showFor("mysql", ins, gooq.MySQL())

	fmt.Println("\nUPDATE and DELETE:")
	show("update", gooq.Update(db.Book).
		Set(db.Book.Price.Set(29.99)).
		Where(db.Book.Id.EQ(1)))
	show("delete", gooq.DeleteFrom(db.Book).
		Where(db.Book.Price.LT(5)).
		Returning(db.Book.Id))

	fmt.Println("\nComposable conditions stored in a variable:")
	cheap := db.Book.Price.LT(10)
	goBook := db.Book.Title.Like("Go%")
	show("combined", gooq.Select1(db.Book.Id).
		From(db.Book).
		Where(cheap.And(goBook)))
}
