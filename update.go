package gooq

import (
	"context"
	"database/sql"
)

// updateStmt is the UPDATE abstract syntax tree node.
type updateStmt struct {
	tableName   string
	assignments []node
	where       node
	returning   *returningClause
}

func (s *updateStmt) render(b *builder) {
	b.writeString("UPDATE ")
	b.writeIdentifier(s.tableName)
	b.writeString(" SET ")
	renderList(b, s.assignments)
	if s.where != nil {
		b.writeString(" WHERE ")
		s.where.render(b)
	}
	if s.returning != nil {
		s.returning.render(b)
	}
}

// UpdateSetStep is the entry state after UPDATE table.
type UpdateSetStep interface {
	Set(a Assignment) UpdateSetMoreStep
}

// UpdateSetMoreStep accepts further assignments or a WHERE clause.
type UpdateSetMoreStep interface {
	UpdateWhereStep
	Set(a Assignment) UpdateSetMoreStep
}

// UpdateWhereStep allows a WHERE clause or terminal execution.
type UpdateWhereStep interface {
	UpdateReturningStep
	Where(c Condition) UpdateConditionStep
}

// UpdateConditionStep allows extending the WHERE predicate.
type UpdateConditionStep interface {
	UpdateReturningStep
	And(c Condition) UpdateConditionStep
	Or(c Condition) UpdateConditionStep
}

// UpdateReturningStep allows a RETURNING clause before terminal execution.
type UpdateReturningStep interface {
	UpdateFinalStep
	Returning(cols ...AnyField) UpdateFinalStep
}

// UpdateFinalStep is the terminal state.
type UpdateFinalStep interface {
	node
	SQL() (string, []any, error)
	SQLFor(d Dialect) (string, []any, error)
	Using(d Dialect) UpdateFinalStep
	Execute(ctx context.Context, db Querier) (sql.Result, error)
}

// updateBuilder implements the entire UPDATE step chain.
type updateBuilder struct {
	stmt    *updateStmt
	dialect Dialect
}

// Update begins an UPDATE of the given table.
func Update(t Table) UpdateSetStep {
	return &updateBuilder{
		stmt:    &updateStmt{tableName: t.TableName()},
		dialect: Postgres(),
	}
}

func (b *updateBuilder) render(bld *builder) { b.stmt.render(bld) }

func (b *updateBuilder) Set(a Assignment) UpdateSetMoreStep {
	b.stmt.assignments = append(b.stmt.assignments, a)
	return b
}

func (b *updateBuilder) Where(c Condition) UpdateConditionStep {
	b.stmt.where = c
	return b
}

func (b *updateBuilder) And(c Condition) UpdateConditionStep {
	b.stmt.where = &boolPredicate{op: "AND", parts: []node{b.stmt.where, c}}
	return b
}

func (b *updateBuilder) Or(c Condition) UpdateConditionStep {
	b.stmt.where = &boolPredicate{op: "OR", parts: []node{b.stmt.where, c}}
	return b
}

func (b *updateBuilder) Returning(cols ...AnyField) UpdateFinalStep {
	b.stmt.returning = newReturning(cols)
	return b
}

func (b *updateBuilder) Using(d Dialect) UpdateFinalStep {
	b.dialect = d
	return b
}

func (b *updateBuilder) SQL() (string, []any, error) { return b.SQLFor(b.dialect) }

func (b *updateBuilder) SQLFor(d Dialect) (string, []any, error) {
	bld := newBuilder(d)
	b.stmt.render(bld)
	return bld.result()
}

func (b *updateBuilder) Execute(ctx context.Context, db Querier) (sql.Result, error) {
	query, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return db.ExecContext(ctx, query, args...)
}
