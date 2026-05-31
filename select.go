package gooq

import (
	"context"
	"database/sql"
)

// The SELECT step interfaces encode the legal clause order in the type system.
// Each method returns the interface describing the clauses that may legally
// follow, and optional clauses are skipped by embedding a later step interface.
// A caller therefore cannot, for example, place WHERE after GROUP BY, but may
// omit WHERE entirely.

// SelectFromStep is the state immediately after the projection is chosen.
// Distinct may be applied here, before the FROM clause, and returns the same
// step so the projection can still be followed by From.
type SelectFromStep[R any] interface {
	From(t Table) SelectJoinStep[R]
	// Distinct marks the query as SELECT DISTINCT. It is idempotent and may be
	// called before From.
	Distinct() SelectFromStep[R]
}

// SelectJoinStep allows joining further tables or, by embedding
// SelectWhereStep, skipping straight to WHERE and beyond.
type SelectJoinStep[R any] interface {
	SelectWhereStep[R]
	Join(t Table) SelectOnStep[R]
	InnerJoin(t Table) SelectOnStep[R]
	LeftJoin(t Table) SelectOnStep[R]
	RightJoin(t Table) SelectOnStep[R]
}

// SelectOnStep requires the join condition before any further clause.
type SelectOnStep[R any] interface {
	On(c Condition) SelectJoinStep[R]
}

// SelectWhereStep allows a WHERE clause or, by embedding SelectGroupByStep,
// skipping it.
type SelectWhereStep[R any] interface {
	SelectGroupByStep[R]
	Where(c Condition) SelectConditionStep[R]
}

// SelectConditionStep allows extending the WHERE predicate with AND/OR.
type SelectConditionStep[R any] interface {
	SelectGroupByStep[R]
	And(c Condition) SelectConditionStep[R]
	Or(c Condition) SelectConditionStep[R]
}

// SelectGroupByStep allows a GROUP BY clause or skipping to ORDER BY and limit.
type SelectGroupByStep[R any] interface {
	SelectOrderByStep[R]
	GroupBy(fields ...AnyField) SelectHavingStep[R]
}

// SelectHavingStep allows a HAVING clause after GROUP BY.
type SelectHavingStep[R any] interface {
	SelectOrderByStep[R]
	Having(c Condition) SelectOrderByStep[R]
}

// SelectOrderByStep allows an ORDER BY clause or skipping to limit.
type SelectOrderByStep[R any] interface {
	SelectLimitStep[R]
	OrderBy(orders ...OrderField) SelectLimitStep[R]
}

// SelectLimitStep allows a LIMIT and/or OFFSET clause or terminal execution.
// Both Limit and Offset return SelectLimitStep so they may be chained in either
// order or used independently (a bare OFFSET is legal).
type SelectLimitStep[R any] interface {
	SelectFinalStep[R]
	Limit(n int64) SelectLimitStep[R]
	Offset(n int64) SelectLimitStep[R]
}

// SelectFinalStep is the terminal state: render the SQL or execute the query.
type SelectFinalStep[R any] interface {
	node
	// Fetch executes the query and returns every row mapped to R.
	Fetch(ctx context.Context, db Querier) ([]R, error)
	// FetchOne returns the single row, the zero value when no row matches, or
	// ErrTooManyRows when more than one row matches.
	FetchOne(ctx context.Context, db Querier) (R, error)
	// FetchSingle returns the single row, sql.ErrNoRows when none matches, or
	// ErrTooManyRows when more than one matches.
	FetchSingle(ctx context.Context, db Querier) (R, error)
	// FetchOptional returns the single row and true, the zero value and false
	// when no row matches, or ErrTooManyRows when more than one row matches.
	FetchOptional(ctx context.Context, db Querier) (R, bool, error)
	// SQL renders the query using the dialect bound to the query (PostgreSQL by
	// default, or whatever was selected via Using).
	SQL() (string, []any, error)
	// SQLFor renders the query using an explicit dialect.
	SQLFor(d Dialect) (string, []any, error)
	// Using selects the dialect used by SQL, Fetch, FetchOne, and FetchSingle.
	Using(d Dialect) SelectFinalStep[R]
}

// selectStmt is the SELECT abstract syntax tree node.
type selectStmt struct {
	distinct   bool
	projection []node
	from       node
	joins      []*joinClause
	where      node
	groupBy    []node
	having     node
	orderBy    []node
	limit      *int64
	offset     *int64
}

func (s *selectStmt) render(b *builder) {
	b.writeString("SELECT ")
	if s.distinct {
		b.writeString("DISTINCT ")
	}
	b.declareAlias = true
	renderList(b, s.projection)
	b.declareAlias = false

	if s.from != nil {
		b.writeString(" FROM ")
		s.from.render(b)
	}
	for _, j := range s.joins {
		j.render(b)
	}
	if s.where != nil {
		b.writeString(" WHERE ")
		s.where.render(b)
	}
	if len(s.groupBy) > 0 {
		b.writeString(" GROUP BY ")
		renderList(b, s.groupBy)
	}
	if s.having != nil {
		b.writeString(" HAVING ")
		s.having.render(b)
	}
	if len(s.orderBy) > 0 {
		b.writeString(" ORDER BY ")
		renderList(b, s.orderBy)
	}
	b.dialect.renderLimit(b, s.limit, s.offset)
}

// joinClause is a single JOIN node.
type joinClause struct {
	kind string // "JOIN", "INNER JOIN", "LEFT JOIN", "RIGHT JOIN"
	tbl  node
	on   node
}

func (j *joinClause) render(b *builder) {
	b.writeString(" ")
	b.writeString(j.kind)
	b.writeString(" ")
	j.tbl.render(b)
	if j.on != nil {
		b.writeString(" ON ")
		j.on.render(b)
	}
}

// selectBuilder is the single concrete type implementing the entire SELECT step
// chain. Methods mutate the underlying statement and return the receiver typed
// as the next legal step. The scan closure, captured by the generic SelectN
// constructor, maps a result row to R.
type selectBuilder[R any] struct {
	stmt        *selectStmt
	scan        func(*sql.Rows) (R, error)
	dialect     Dialect
	pendingJoin *joinClause
}

func newSelect[R any](projection []node, scan func(*sql.Rows) (R, error)) *selectBuilder[R] {
	return &selectBuilder[R]{
		stmt:    &selectStmt{projection: projection},
		scan:    scan,
		dialect: Postgres(),
	}
}

func (s *selectBuilder[R]) render(b *builder) { s.stmt.render(b) }

func (s *selectBuilder[R]) Distinct() SelectFromStep[R] {
	s.stmt.distinct = true
	return s
}

func (s *selectBuilder[R]) From(t Table) SelectJoinStep[R] {
	s.stmt.from = t
	return s
}

func (s *selectBuilder[R]) addJoin(kind string, t Table) SelectOnStep[R] {
	j := &joinClause{kind: kind, tbl: t}
	s.stmt.joins = append(s.stmt.joins, j)
	s.pendingJoin = j
	return s
}

func (s *selectBuilder[R]) Join(t Table) SelectOnStep[R]      { return s.addJoin("JOIN", t) }
func (s *selectBuilder[R]) InnerJoin(t Table) SelectOnStep[R] { return s.addJoin("INNER JOIN", t) }
func (s *selectBuilder[R]) LeftJoin(t Table) SelectOnStep[R]  { return s.addJoin("LEFT JOIN", t) }
func (s *selectBuilder[R]) RightJoin(t Table) SelectOnStep[R] { return s.addJoin("RIGHT JOIN", t) }

func (s *selectBuilder[R]) On(c Condition) SelectJoinStep[R] {
	if s.pendingJoin != nil {
		s.pendingJoin.on = c
	}
	return s
}

func (s *selectBuilder[R]) Where(c Condition) SelectConditionStep[R] {
	s.stmt.where = c
	return s
}

func (s *selectBuilder[R]) And(c Condition) SelectConditionStep[R] {
	s.stmt.where = &boolPredicate{op: "AND", parts: []node{s.stmt.where, c}}
	return s
}

func (s *selectBuilder[R]) Or(c Condition) SelectConditionStep[R] {
	s.stmt.where = &boolPredicate{op: "OR", parts: []node{s.stmt.where, c}}
	return s
}

func (s *selectBuilder[R]) GroupBy(fields ...AnyField) SelectHavingStep[R] {
	for _, f := range fields {
		s.stmt.groupBy = append(s.stmt.groupBy, f)
	}
	return s
}

func (s *selectBuilder[R]) Having(c Condition) SelectOrderByStep[R] {
	s.stmt.having = c
	return s
}

func (s *selectBuilder[R]) OrderBy(orders ...OrderField) SelectLimitStep[R] {
	for _, o := range orders {
		s.stmt.orderBy = append(s.stmt.orderBy, o)
	}
	return s
}

func (s *selectBuilder[R]) Limit(n int64) SelectLimitStep[R] {
	s.stmt.limit = &n
	return s
}

func (s *selectBuilder[R]) Offset(n int64) SelectLimitStep[R] {
	s.stmt.offset = &n
	return s
}

func (s *selectBuilder[R]) Using(d Dialect) SelectFinalStep[R] {
	s.dialect = d
	return s
}

func (s *selectBuilder[R]) SQL() (string, []any, error) {
	return s.SQLFor(s.dialect)
}

func (s *selectBuilder[R]) SQLFor(d Dialect) (string, []any, error) {
	b := newBuilder(d)
	s.stmt.render(b)
	return b.result()
}

func (s *selectBuilder[R]) Fetch(ctx context.Context, db Querier) ([]R, error) {
	query, args, err := s.SQL()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []R
	for rows.Next() {
		r, err := s.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *selectBuilder[R]) FetchOne(ctx context.Context, db Querier) (R, error) {
	var zero R
	rows, err := s.Fetch(ctx, db)
	if err != nil {
		return zero, err
	}
	switch len(rows) {
	case 0:
		return zero, nil
	case 1:
		return rows[0], nil
	default:
		return zero, ErrTooManyRows
	}
}

func (s *selectBuilder[R]) FetchSingle(ctx context.Context, db Querier) (R, error) {
	var zero R
	rows, err := s.Fetch(ctx, db)
	if err != nil {
		return zero, err
	}
	switch len(rows) {
	case 0:
		return zero, sql.ErrNoRows
	case 1:
		return rows[0], nil
	default:
		return zero, ErrTooManyRows
	}
}

// FetchOptional returns the single row and true, the zero value and false when
// the query matches no row, or ErrTooManyRows when it matches more than one.
func (s *selectBuilder[R]) FetchOptional(ctx context.Context, db Querier) (R, bool, error) {
	var zero R
	rows, err := s.Fetch(ctx, db)
	if err != nil {
		return zero, false, err
	}
	switch len(rows) {
	case 0:
		return zero, false, nil
	case 1:
		return rows[0], true, nil
	default:
		return zero, false, ErrTooManyRows
	}
}
