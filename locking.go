package gooq

// locking.go implements the SELECT row-locking clause (FOR UPDATE / FOR SHARE,
// optionally SKIP LOCKED). The clause is recorded on the statement and rendered
// after LIMIT and OFFSET by the active dialect: PostgreSQL emits it, while
// SQLite omits it because it has no row-locking clause.

func (s *selectBuilder[R]) ForUpdate() SelectLockStep[R] {
	s.stmt.lockMode = "FOR UPDATE"
	return s
}

func (s *selectBuilder[R]) ForShare() SelectLockStep[R] {
	s.stmt.lockMode = "FOR SHARE"
	return s
}

func (s *selectBuilder[R]) SkipLocked() SelectLockStep[R] {
	s.stmt.skipLocked = true
	return s
}
