package gooq

// RecordN are positional, fully typed row types that preserve each projected
// column's Go type by position, mirroring jOOQ's Record1..Record22. Records 1
// through 5 are written by hand; records 6 through 22 are generated into
// record_gen.go.

// Record1 holds one typed column value.
type Record1[T1 any] struct {
	V1 T1
}

// Values returns the column values in projection order.
func (r Record1[T1]) Values() []any { return []any{r.V1} }

// Get returns the i-th column value (zero-based), or nil when out of range.
func (r Record1[T1]) Get(i int) any {
	if i == 0 {
		return r.V1
	}
	return nil
}

// Record2 holds two typed column values.
type Record2[T1, T2 any] struct {
	V1 T1
	V2 T2
}

func (r Record2[T1, T2]) Values() []any { return []any{r.V1, r.V2} }

func (r Record2[T1, T2]) Get(i int) any {
	switch i {
	case 0:
		return r.V1
	case 1:
		return r.V2
	}
	return nil
}

// Record3 holds three typed column values.
type Record3[T1, T2, T3 any] struct {
	V1 T1
	V2 T2
	V3 T3
}

func (r Record3[T1, T2, T3]) Values() []any { return []any{r.V1, r.V2, r.V3} }

func (r Record3[T1, T2, T3]) Get(i int) any {
	switch i {
	case 0:
		return r.V1
	case 1:
		return r.V2
	case 2:
		return r.V3
	}
	return nil
}

// Record4 holds four typed column values.
type Record4[T1, T2, T3, T4 any] struct {
	V1 T1
	V2 T2
	V3 T3
	V4 T4
}

func (r Record4[T1, T2, T3, T4]) Values() []any { return []any{r.V1, r.V2, r.V3, r.V4} }

func (r Record4[T1, T2, T3, T4]) Get(i int) any {
	switch i {
	case 0:
		return r.V1
	case 1:
		return r.V2
	case 2:
		return r.V3
	case 3:
		return r.V4
	}
	return nil
}

// Record5 holds five typed column values.
type Record5[T1, T2, T3, T4, T5 any] struct {
	V1 T1
	V2 T2
	V3 T3
	V4 T4
	V5 T5
}

func (r Record5[T1, T2, T3, T4, T5]) Values() []any { return []any{r.V1, r.V2, r.V3, r.V4, r.V5} }

func (r Record5[T1, T2, T3, T4, T5]) Get(i int) any {
	switch i {
	case 0:
		return r.V1
	case 1:
		return r.V2
	case 2:
		return r.V3
	case 3:
		return r.V4
	case 4:
		return r.V5
	}
	return nil
}
