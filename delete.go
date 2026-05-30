package gooq

import (
	"context"
	"database/sql"
)

// deleteStmt is the DELETE abstract syntax tree node.
type deleteStmt struct {
	tableName string
	where     node
	returning *returningClause
}

func (s *deleteStmt) render(b *builder) {
	b.writeString("DELETE FROM ")
	b.writeIdentifier(s.tableName)
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
