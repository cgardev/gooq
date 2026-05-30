package gooq

import "database/sql"

// SelectN begin a typed SELECT whose result rows are RecordN values. Because Go
// methods cannot declare their own type parameters, these are top-level generic
// functions rather than methods, exactly as the design requires. Functions for
// arities 1 through 5 are written by hand; 6 through 22 are generated into
// select_gen.go.

// Select1 begins a SELECT projecting one typed column.
func Select1[T1 any](f1 Field[T1]) SelectFromStep[Record1[T1]] {
	return newSelect([]node{f1}, func(rows *sql.Rows) (Record1[T1], error) {
		var r Record1[T1]
		err := rows.Scan(&r.V1)
		return r, err
	})
}

// Select2 begins a SELECT projecting two typed columns.
func Select2[T1, T2 any](f1 Field[T1], f2 Field[T2]) SelectFromStep[Record2[T1, T2]] {
	return newSelect([]node{f1, f2}, func(rows *sql.Rows) (Record2[T1, T2], error) {
		var r Record2[T1, T2]
		err := rows.Scan(&r.V1, &r.V2)
		return r, err
	})
}

// Select3 begins a SELECT projecting three typed columns.
func Select3[T1, T2, T3 any](f1 Field[T1], f2 Field[T2], f3 Field[T3]) SelectFromStep[Record3[T1, T2, T3]] {
	return newSelect([]node{f1, f2, f3}, func(rows *sql.Rows) (Record3[T1, T2, T3], error) {
		var r Record3[T1, T2, T3]
		err := rows.Scan(&r.V1, &r.V2, &r.V3)
		return r, err
	})
}

// Select4 begins a SELECT projecting four typed columns.
func Select4[T1, T2, T3, T4 any](f1 Field[T1], f2 Field[T2], f3 Field[T3], f4 Field[T4]) SelectFromStep[Record4[T1, T2, T3, T4]] {
	return newSelect([]node{f1, f2, f3, f4}, func(rows *sql.Rows) (Record4[T1, T2, T3, T4], error) {
		var r Record4[T1, T2, T3, T4]
		err := rows.Scan(&r.V1, &r.V2, &r.V3, &r.V4)
		return r, err
	})
}

// Select5 begins a SELECT projecting five typed columns.
func Select5[T1, T2, T3, T4, T5 any](f1 Field[T1], f2 Field[T2], f3 Field[T3], f4 Field[T4], f5 Field[T5]) SelectFromStep[Record5[T1, T2, T3, T4, T5]] {
	return newSelect([]node{f1, f2, f3, f4, f5}, func(rows *sql.Rows) (Record5[T1, T2, T3, T4, T5], error) {
		var r Record5[T1, T2, T3, T4, T5]
		err := rows.Scan(&r.V1, &r.V2, &r.V3, &r.V4, &r.V5)
		return r, err
	})
}
