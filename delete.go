package gooq

import (
	"context"
	"database/sql"
	"errors"
)

// ErrUsingUnsupported is recorded when a DELETE ... USING clause is rendered for
// a dialect that does not support it. Only PostgreSQL supports DELETE ... USING;
// SQLite does not.
var ErrUsingUnsupported = errors.New("gooq: DELETE ... USING is not supported by this dialect")

// deleteStmt is the DELETE abstract syntax tree node.
type deleteStmt struct {
	tableName string
	// usingTable, when set, names a further table joined into the DELETE through
	// a USING clause ("DELETE FROM t USING other WHERE ..."). Only PostgreSQL
	// supports DELETE ... USING; SQLite has no such clause, so rendering it for
	// the SQLite dialect records ErrUsingUnsupported.
	usingTable string
	where      node
	returning  *returningClause
}

func (s *deleteStmt) render(b *builder) {
	b.writeString("DELETE FROM ")
	b.writeIdentifier(s.tableName)
	if s.usingTable != "" {
		if !b.dialect.supportsDeleteUsing() {
			b.setError(ErrUsingUnsupported)
			return
		}
		b.writeString(" USING ")
		b.writeIdentifier(s.usingTable)
	}
	if s.where != nil {
		b.writeString(" WHERE ")
		s.where.render(b)
	}
	if s.returning != nil {
		s.returning.render(b)
	}
}

// DeleteWhereStep is the entry state after DELETE FROM table.
type DeleteWhereStep interface {
	DeleteReturningStep
	Where(c Condition) DeleteConditionStep
	// UsingTable joins a further table into the DELETE through a USING clause,
	// rendered before WHERE. Only PostgreSQL supports DELETE ... USING; the
	// SQLite dialect records ErrUsingUnsupported. This clause is named
	// UsingTable rather than Using because Using is reserved on the terminal
	// step for selecting the rendering dialect.
	UsingTable(t Table) DeleteWhereStep
}

// DeleteConditionStep allows extending the WHERE predicate.
type DeleteConditionStep interface {
	DeleteReturningStep
	And(c Condition) DeleteConditionStep
	Or(c Condition) DeleteConditionStep
}

// DeleteReturningStep allows a RETURNING clause before terminal execution.
type DeleteReturningStep interface {
	DeleteFinalStep
	Returning(cols ...AnyField) DeleteFinalStep
}

// DeleteFinalStep is the terminal state.
type DeleteFinalStep interface {
	node
	SQL() (string, []any, error)
	SQLFor(d Dialect) (string, []any, error)
	Using(d Dialect) DeleteFinalStep
	Execute(ctx context.Context, db Querier) (sql.Result, error)
}

// deleteBuilder implements the entire DELETE step chain.
type deleteBuilder struct {
	stmt    *deleteStmt
	dialect Dialect
}

// DeleteFrom begins a DELETE from the given table.
func DeleteFrom(t Table) DeleteWhereStep {
	return &deleteBuilder{
		stmt:    &deleteStmt{tableName: t.TableName()},
		dialect: Postgres(),
	}
}

func (b *deleteBuilder) render(bld *builder) { b.stmt.render(bld) }

func (b *deleteBuilder) UsingTable(t Table) DeleteWhereStep {
	b.stmt.usingTable = t.TableName()
	return b
}

func (b *deleteBuilder) Where(c Condition) DeleteConditionStep {
	b.stmt.where = c
	return b
}

func (b *deleteBuilder) And(c Condition) DeleteConditionStep {
	b.stmt.where = &boolPredicate{op: "AND", parts: []node{b.stmt.where, c}}
	return b
}

func (b *deleteBuilder) Or(c Condition) DeleteConditionStep {
	b.stmt.where = &boolPredicate{op: "OR", parts: []node{b.stmt.where, c}}
	return b
}

func (b *deleteBuilder) Returning(cols ...AnyField) DeleteFinalStep {
	b.stmt.returning = newReturning(cols)
	return b
}

func (b *deleteBuilder) Using(d Dialect) DeleteFinalStep {
	b.dialect = d
	return b
}

func (b *deleteBuilder) SQL() (string, []any, error) { return b.SQLFor(b.dialect) }

func (b *deleteBuilder) SQLFor(d Dialect) (string, []any, error) {
	bld := newBuilder(d)
	b.stmt.render(bld)
	return bld.result()
}

func (b *deleteBuilder) Execute(ctx context.Context, db Querier) (sql.Result, error) {
	query, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return db.ExecContext(ctx, query, args...)
}
