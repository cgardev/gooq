package gooq

import (
	"context"
	"database/sql"
)

// upsertClause is the conflict-resolution tail of an INSERT statement. Its
// rendering is delegated to the active dialect, which emits the PostgreSQL and
// SQLite ON CONFLICT ... DO ... form.
type upsertClause struct {
	conflictCols []string
	doNothing    bool
	assignments  []Assignment
}

// returningClause renders a RETURNING projection, or records an error when the
// dialect does not support it. Columns are emitted unqualified, which is the
// conventional and portable form. An empty column list renders RETURNING *.
type returningClause struct {
	cols []string
}

func (r *returningClause) render(b *builder) {
	if !b.dialect.supportsReturning() {
		b.setError(ErrReturningUnsupported)
		return
	}
	b.writeString(" RETURNING ")
	if len(r.cols) == 0 {
		b.writeString("*")
		return
	}
	for i, c := range r.cols {
		if i > 0 {
			b.writeString(", ")
		}
		b.writeIdentifier(c)
	}
}

// newReturning builds a returning clause from a column list.
func newReturning(cols []AnyField) *returningClause {
	rc := &returningClause{}
	for _, c := range cols {
		rc.cols = append(rc.cols, c.Name())
	}
	return rc
}

// insertStmt is the INSERT abstract syntax tree node.
type insertStmt struct {
	tableName     string
	columns       []string
	rows          [][]node
	defaultValues bool
	upsert        *upsertClause
	returning     *returningClause
}

func (s *insertStmt) render(b *builder) {
	b.writeString("INSERT INTO ")
	b.writeIdentifier(s.tableName)

	if s.defaultValues {
		b.writeString(" DEFAULT VALUES")
		s.renderTail(b)
		return
	}

	if len(s.columns) == 0 || len(s.rows) == 0 {
		b.setError(ErrEmptyInsert)
		return
	}

	b.writeString(" (")
	for i, c := range s.columns {
		if i > 0 {
			b.writeString(", ")
		}
		b.writeIdentifier(c)
	}
	b.writeString(") VALUES ")

	for ri, row := range s.rows {
		if len(row) != len(s.columns) {
			b.setError(ErrColumnValueMismatch)
			return
		}
		if ri > 0 {
			b.writeString(", ")
		}
		b.writeString("(")
		renderList(b, row)
		b.writeString(")")
	}

	s.renderTail(b)
}

func (s *insertStmt) renderTail(b *builder) {
	if s.upsert != nil {
		b.dialect.renderUpsert(b, s.upsert)
	}
	if s.returning != nil {
		s.returning.render(b)
	}
}

// InsertSetStep is the entry state after INSERT INTO.
type InsertSetStep interface {
	Columns(cols ...AnyField) InsertValuesStep
	Set(a Assignment) InsertSetMoreStep
	DefaultValues() InsertFinalStep
}

// InsertValuesStep accepts one or more value rows.
type InsertValuesStep interface {
	InsertOnConflictStep
	Values(values ...any) InsertValuesStep
}

// InsertSetMoreStep accepts further column assignments.
type InsertSetMoreStep interface {
	InsertOnConflictStep
	Set(a Assignment) InsertSetMoreStep
}

// InsertOnConflictStep allows attaching conflict-resolution behavior.
type InsertOnConflictStep interface {
	InsertReturningStep
	OnConflict(cols ...AnyField) InsertConflictActionStep
	OnConflictDoNothing() InsertReturningStep
	OnDuplicateKeyUpdate(assignments ...Assignment) InsertReturningStep
}

// InsertConflictActionStep chooses the action for an ON CONFLICT target.
type InsertConflictActionStep interface {
	DoNothing() InsertReturningStep
	DoUpdateSet(assignments ...Assignment) InsertReturningStep
}

// InsertReturningStep allows a RETURNING clause before terminal execution.
type InsertReturningStep interface {
	InsertFinalStep
	Returning(cols ...AnyField) InsertFinalStep
}

// InsertFinalStep is the terminal state.
type InsertFinalStep interface {
	node
	SQL() (string, []any, error)
	SQLFor(d Dialect) (string, []any, error)
	Using(d Dialect) InsertFinalStep
	Execute(ctx context.Context, db Querier) (sql.Result, error)
}

// insertBuilder implements the entire INSERT step chain.
type insertBuilder struct {
	stmt    *insertStmt
	dialect Dialect
}

// InsertInto begins an INSERT into the given table.
func InsertInto(t Table) InsertSetStep {
	return &insertBuilder{
		stmt:    &insertStmt{tableName: t.TableName()},
		dialect: Postgres(),
	}
}

func (b *insertBuilder) render(bld *builder) { b.stmt.render(bld) }

func (b *insertBuilder) Columns(cols ...AnyField) InsertValuesStep {
	for _, c := range cols {
		b.stmt.columns = append(b.stmt.columns, c.Name())
	}
	return b
}

func (b *insertBuilder) Values(values ...any) InsertValuesStep {
	row := make([]node, len(values))
	for i, v := range values {
		row[i] = bindOf(v)
	}
	b.stmt.rows = append(b.stmt.rows, row)
	return b
}

func (b *insertBuilder) Set(a Assignment) InsertSetMoreStep {
	an := a.(*assignmentNode)
	b.stmt.columns = append(b.stmt.columns, an.column)
	if len(b.stmt.rows) == 0 {
		b.stmt.rows = append(b.stmt.rows, nil)
	}
	b.stmt.rows[0] = append(b.stmt.rows[0], an.val)
	return b
}

func (b *insertBuilder) DefaultValues() InsertFinalStep {
	b.stmt.defaultValues = true
	return b
}

func columnNames(cols []AnyField) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name()
	}
	return names
}

func (b *insertBuilder) OnConflict(cols ...AnyField) InsertConflictActionStep {
	b.stmt.upsert = &upsertClause{conflictCols: columnNames(cols)}
	return b
}

func (b *insertBuilder) OnConflictDoNothing() InsertReturningStep {
	b.stmt.upsert = &upsertClause{doNothing: true}
	return b
}

func (b *insertBuilder) OnDuplicateKeyUpdate(assignments ...Assignment) InsertReturningStep {
	b.stmt.upsert = &upsertClause{assignments: assignments}
	return b
}

func (b *insertBuilder) DoNothing() InsertReturningStep {
	b.stmt.upsert.doNothing = true
	return b
}

func (b *insertBuilder) DoUpdateSet(assignments ...Assignment) InsertReturningStep {
	b.stmt.upsert.assignments = assignments
	return b
}

func (b *insertBuilder) Returning(cols ...AnyField) InsertFinalStep {
	b.stmt.returning = newReturning(cols)
	return b
}

func (b *insertBuilder) Using(d Dialect) InsertFinalStep {
	b.dialect = d
	return b
}

func (b *insertBuilder) SQL() (string, []any, error) { return b.SQLFor(b.dialect) }

func (b *insertBuilder) SQLFor(d Dialect) (string, []any, error) {
	bld := newBuilder(d)
	b.stmt.render(bld)
	return bld.result()
}

func (b *insertBuilder) Execute(ctx context.Context, db Querier) (sql.Result, error) {
	query, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return db.ExecContext(ctx, query, args...)
}
