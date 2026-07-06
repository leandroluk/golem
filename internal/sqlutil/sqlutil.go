// Package sqlutil holds tiny helpers shared by every database/sql-based
// adapter (driver/mysql, driver/mssql, driver/oracle, driver/sqlite —
// driver/postgres is pgx-native and doesn't need this).
package sqlutil

import "strings"

// rowReturningKeywords are the leading keywords of a statement that
// produces a row-returning result set through database/sql's QueryContext
// — as opposed to a write statement (INSERT/UPDATE/DELETE/...) whose real
// affected-row-count is only available via ExecContext's sql.Result.
var rowReturningKeywords = []string{
	"SELECT", "WITH", "SHOW", "EXPLAIN", "DESCRIBE", "DESC", "PRAGMA",
}

// IsRowReturning reports whether sql looks like a statement that returns a
// row set (a SELECT and its common relatives), as opposed to a write
// statement. This is a heuristic, not a parser — good enough to pick
// QueryContext vs. ExecContext in Dialect.ExecRaw, since database/sql
// itself offers no unified "run either kind, tell me what happened"
// primitive: QueryContext gives back rows but no real RowsAffected() for a
// write, ExecContext gives a real RowsAffected() but no row data for a
// read. Confirmed necessary via a real bug: every database/sql-based
// adapter's ExecRaw used to always call QueryContext, so
// golem.DataSource.Exec("UPDATE ...") always reported 0 rows affected
// (collectRows sees an empty row set for a non-row-returning statement) —
// driver/postgres never had this problem because pgx's Rows expose a
// CommandTag with the real affected count even via its own Query path.
func IsRowReturning(sql string) bool {
	trimmed := strings.TrimSpace(sql)
	// Skip a leading parenthesis, e.g. "(SELECT 1) UNION (SELECT 2)".
	trimmed = strings.TrimLeft(trimmed, "(")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}
	first := strings.ToUpper(fields[0])
	for _, kw := range rowReturningKeywords {
		if first == kw {
			return true
		}
	}
	return false
}
