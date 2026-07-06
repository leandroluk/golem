package sqlite

// resolveDSN builds a modernc.org/sqlite connection string from o.Path.
// Unlike driver/postgres's/driver/mysql's resolveDSN, there is no
// network address space to parse and nothing here can fail — so this
// returns a plain string, not (string, error).
//
// Path == "" defaults to ":memory:". When the path is ":memory:", the
// query string is prefixed with "cache=shared" — the standard SQLite URI
// idiom for making a named in-memory database visible to more than one
// connection within the same process, instead of each new pooled
// connection getting its own private, empty in-memory database.
//
// Three settings are always forced, never user-configurable (internal
// implementation requirements, matching driver/mysql's forced
// ParseTime=true):
//   - foreign_keys(1): SQLite defaults FK enforcement OFF for backward
//     compatibility; without this, cascade/restrict/set-null behavior
//     would silently not be enforced at all.
//   - busy_timeout(5000): cheap insurance against spurious SQLITE_BUSY.
//   - _time_format=sqlite: repository never calls golem.Dialect.Bind (it's
//     dead code, same as every other adapter) — it hands the driver a raw
//     Go time.Time value directly for any date/datetime/time field, so the
//     driver's own serialization format is what actually lands in the
//     database. Without this, modernc.org/sqlite falls back to time.Time's
//     default String() format, which isn't parseable back out (see
//     dialect.go's normalizeCell, needed for the one case — TIME-declared
//     columns — the driver doesn't already auto-convert back to time.Time
//     on Scan).
func resolveDSN(o *Options) string {
	path := o.Path
	if path == "" {
		path = ":memory:"
	}

	query := "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_time_format=sqlite"
	if path == ":memory:" {
		query = "cache=shared&" + query
	}

	return "file:" + path + "?" + query
}
