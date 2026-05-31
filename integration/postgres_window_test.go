package integration

import (
	"context"
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// postgres_window_test.go exercises the ranking and offset window functions
// against the real PostgreSQL container. Each test seeds the standard library
// and then adds a few extra reviews inside the per-test transaction so the
// partitions carry ties, which distinguishes ROW_NUMBER, RANK, and DENSE_RANK
// and gives LEAD/LAG meaningful neighbours.

// seedRankReviews inserts additional reviews for the Go book so a single
// partition holds five ratings with a deliberate tie: 5, 5, 4, 4, 3. This lets
// the ranking functions diverge in a way the assertions can pin down.
func seedRankReviews(ctx context.Context, t *testing.T, conn gooq.Querier) {
	t.Helper()

	// The seed already gives the Go book two reviews (Ada 5, Linus 4). Add three
	// more so the Go partition becomes [5, 5, 4, 4, 3] once ordered by rating
	// descending.
	_, err := gooq.InsertInto(db.Review).
		Columns(db.Review.Id, db.Review.BookId, db.Review.Reviewer, db.Review.Rating, db.Review.Body).
		Values("c0000000-0000-0000-0000-0000000000a1", bookGo, "Edsger", int64(5), text("Excellent.")).
		Values("c0000000-0000-0000-0000-0000000000a2", bookGo, "Ken", int64(4), text("Good.")).
		Values("c0000000-0000-0000-0000-0000000000a3", bookGo, "Dennis", int64(3), text("Fine.")).
		Execute(ctx, conn)
	noError(t, "seed extra Go reviews", err)
}

func TestPostgresWindowRanking(t *testing.T) {
	t.Run("ROW_NUMBER numbers reviews sequentially within the partition", func(t *testing.T) {
		ctx, tx := library(t)
		seedRankReviews(ctx, t, tx)

		// ROW_NUMBER() OVER (PARTITION BY book_id ORDER BY rating DESC, reviewer ASC)
		// assigns a strictly increasing number per partition with no ties. For the
		// Go partition ordered [Ada 5, Edsger 5, Ken 4, Linus 4, Dennis 3] the
		// numbers are 1..5.
		rowNumber := gooq.RowNumber().Over().
			PartitionBy(db.Review.BookId).
			OrderBy(db.Review.Rating.Desc(), db.Review.Reviewer.Asc()).
			End()

		rows, err := gooq.Select2(db.Review.Reviewer, rowNumber).
			From(db.Review).
			Where(db.Review.BookId.EQ(bookGo)).
			OrderBy(db.Review.Rating.Desc(), db.Review.Reviewer.Asc()).
			Fetch(ctx, tx)
		noError(t, "row number over partition", err)

		equal(t, "row count", len(rows), 5)
		byReviewer := map[string]int64{}
		for _, r := range rows {
			byReviewer[r.V1] = r.V2
		}
		equal(t, "Ada row number", byReviewer["Ada"], int64(1))
		equal(t, "Edsger row number", byReviewer["Edsger"], int64(2))
		equal(t, "Ken row number", byReviewer["Ken"], int64(3))
		equal(t, "Linus row number", byReviewer["Linus"], int64(4))
		equal(t, "Dennis row number", byReviewer["Dennis"], int64(5))
	})

	t.Run("RANK and DENSE_RANK differ on the gap after a tie", func(t *testing.T) {
		ctx, tx := library(t)
		seedRankReviews(ctx, t, tx)

		// Ordered by rating DESC the Go partition is [5, 5, 4, 4, 3].
		//   RANK:       5->1, 5->1, 4->3, 4->3, 3->5 (gaps after ties)
		//   DENSE_RANK: 5->1, 5->1, 4->2, 4->2, 3->3 (no gaps)
		rank := gooq.Rank().Over().
			PartitionBy(db.Review.BookId).
			OrderBy(db.Review.Rating.Desc()).
			End()
		denseRank := gooq.DenseRank().Over().
			PartitionBy(db.Review.BookId).
			OrderBy(db.Review.Rating.Desc()).
			End()

		rows, err := gooq.Select3(db.Review.Rating, rank, denseRank).
			From(db.Review).
			Where(db.Review.BookId.EQ(bookGo)).
			OrderBy(db.Review.Rating.Desc()).
			Fetch(ctx, tx)
		noError(t, "rank and dense rank over partition", err)

		equal(t, "row count", len(rows), 5)
		// Index the (rank, dense rank) pair by rating; both tied rows agree.
		type pair struct{ rank, dense int64 }
		byRating := map[int64]pair{}
		for _, r := range rows {
			byRating[r.V1] = pair{rank: r.V2, dense: r.V3}
		}
		equal(t, "rating 5 rank", byRating[5].rank, int64(1))
		equal(t, "rating 5 dense rank", byRating[5].dense, int64(1))
		equal(t, "rating 4 rank", byRating[4].rank, int64(3))
		equal(t, "rating 4 dense rank", byRating[4].dense, int64(2))
		equal(t, "rating 3 rank", byRating[3].rank, int64(5))
		equal(t, "rating 3 dense rank", byRating[3].dense, int64(3))
	})
}

func TestPostgresWindowOffsets(t *testing.T) {
	t.Run("LEAD and LAG read the neighbouring rating in the ordered partition", func(t *testing.T) {
		ctx, tx := library(t)
		seedRankReviews(ctx, t, tx)

		// Ordered by rating DESC then reviewer ASC the Go partition is:
		//   Ada 5, Edsger 5, Ken 4, Linus 4, Dennis 3.
		// LAG(rating) reads the previous row's rating; LEAD(rating) the next. The
		// boundary rows have no neighbour, so the offset is SQL NULL there; wrapping
		// each window function in COALESCE with a -1 sentinel keeps the projection a
		// plain int64 that always scans, while the interior rows carry the real
		// neighbouring rating.
		lag := gooq.Lag(db.Review.Rating).Over().
			PartitionBy(db.Review.BookId).
			OrderBy(db.Review.Rating.Desc(), db.Review.Reviewer.Asc()).
			End()
		lead := gooq.Lead(db.Review.Rating).Over().
			PartitionBy(db.Review.BookId).
			OrderBy(db.Review.Rating.Desc(), db.Review.Reviewer.Asc()).
			End()
		previous := gooq.Coalesce(lag, int64(-1))
		next := gooq.Coalesce(lead, int64(-1))

		rows, err := gooq.Select3(db.Review.Reviewer, previous, next).
			From(db.Review).
			Where(db.Review.BookId.EQ(bookGo)).
			OrderBy(db.Review.Rating.Desc(), db.Review.Reviewer.Asc()).
			Fetch(ctx, tx)
		noError(t, "lead and lag over partition", err)

		equal(t, "row count", len(rows), 5)

		// The first row has no predecessor and the last row no successor, so those
		// offsets fall back to the -1 sentinel.
		equal(t, "Ada is first", rows[0].V1, "Ada")
		equal(t, "Ada lag is the sentinel", rows[0].V2, int64(-1))
		equal(t, "Edsger lag reads Ada's rating", rows[1].V2, int64(5))
		equal(t, "Ada lead reads Edsger's rating", rows[0].V3, int64(5))
		equal(t, "Ken lead reads Linus's rating", rows[2].V3, int64(4))
		equal(t, "Dennis is last", rows[4].V1, "Dennis")
		equal(t, "Dennis lag reads Linus's rating", rows[4].V2, int64(4))
		equal(t, "Dennis lead is the sentinel", rows[4].V3, int64(-1))
	})
}
