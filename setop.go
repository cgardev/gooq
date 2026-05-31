package gooq

// setop.go implements the SQL set operations UNION, UNION ALL, INTERSECT, and
// EXCEPT over two SELECT queries of the same row type. Both operands render into
// the same builder so their bind arguments interleave in render order, which
// keeps positional placeholders numbered correctly under PostgreSQL.

// setOpStmt renders "left OP right" for a set operation. The operands are the
// SELECT statement nodes of the two combined queries, never re-rendered with a
// fresh builder.
type setOpStmt struct {
	op    string // "UNION", "UNION ALL", "INTERSECT", or "EXCEPT"
	left  node
	right node
}

func (s *setOpStmt) render(b *builder) {
	s.left.render(b)
	b.writeString(" ")
	b.writeString(s.op)
	b.writeString(" ")
	s.right.render(b)
}

// combineSetOp builds a new selectBuilder that renders the set operation between
// the receiver and the other query. The left side's scan closure and dialect
// are preserved so Fetch returns the same RecordN row type, mapping every merged
// row through the left projection.
func (s *selectBuilder[R]) combineSetOp(op string, other SelectFinalStep[R]) SelectFinalStep[R] {
	right, ok := other.(*selectBuilder[R])
	if !ok {
		// Every SelectFinalStep is produced by the SelectN constructors, so the
		// concrete type is always *selectBuilder[R]; this guard keeps the seal.
		return s
	}
	combined := &setOpStmt{op: op, left: s.queryNode(), right: right.queryNode()}
	return &selectBuilder[R]{
		setOp:   combined,
		scan:    s.scan,
		dialect: s.dialect,
	}
}

func (s *selectBuilder[R]) Union(other SelectFinalStep[R]) SelectFinalStep[R] {
	return s.combineSetOp("UNION", other)
}

func (s *selectBuilder[R]) UnionAll(other SelectFinalStep[R]) SelectFinalStep[R] {
	return s.combineSetOp("UNION ALL", other)
}

func (s *selectBuilder[R]) Intersect(other SelectFinalStep[R]) SelectFinalStep[R] {
	return s.combineSetOp("INTERSECT", other)
}

func (s *selectBuilder[R]) Except(other SelectFinalStep[R]) SelectFinalStep[R] {
	return s.combineSetOp("EXCEPT", other)
}
