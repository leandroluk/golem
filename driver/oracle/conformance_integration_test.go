//go:build integration

package oracle

import (
	"testing"

	"github.com/leandroluk/golem/internal/dialecttest"
)

// oracleConformanceSchema supplies dialecttest's fixed logical tables as
// real Oracle DDL, per INSIGHT.md's type table (BOOLEAN->NUMBER(1),
// SMALLINT/INTEGER/BIGINT->NUMBER(5/10/19), DECIMAL(p,s)->NUMBER(p,s),
// TEXT/JSON->CLOB, UUID->VARCHAR2(36) — design decision, not RAW(16), see
// design.md). GENERATED ALWAYS AS IDENTITY is Oracle 12c+'s auto-increment
// primary key. The integration-test Oracle container is recreated fresh
// by `task test-integration` on every run, so plain CREATE TABLE (no
// cross-run cleanup) is fine, same reasoning as driver/mssql's own
// conformance schema.
//
// "duration" (golem's "time" kind) uses TIMESTAMP, NOT INSIGHT.md's
// suggested INTERVAL DAY TO SECOND — a design correction made during T10's
// own verification run: go-ora's parameter encoder only knows how to bind
// a Go time.Time as Oracle's TIMESTAMP WITH TIME ZONE, and Oracle refuses
// the implicit conversion into an INTERVAL DAY TO SECOND column
// (ORA-00932). Since golem's "time" kind is always represented as a plain
// time.Time (no dedicated duration type), and repository.Insert/Update
// never call Dialect.Bind in the real path (AD-037), there's no
// column-type-aware hook available to reformat just this one column —
// TIMESTAMP is the pragmatic fix: it accepts time.Time natively (same as
// the "date"/"datetime" kinds), at the cost of losing INTERVAL's
// multi-day-duration range, a real but already-accepted-elsewhere class of
// trade-off (matches SQLite's DECIMAL-as-REAL precision loss, MySQL's
// TIME range limit).
//
// Every table/column name here is double-quoted — confirmed necessary via
// a real ORA-00942 "table or view does not exist" error during T10's own
// verification run: unlike Postgres (unquoted identifiers fold to
// lowercase, matching this adapter's own always-quote-and-preserve-case
// quoteIdent), Oracle folds UNQUOTED identifiers to UPPERCASE — so an
// unquoted `CREATE TABLE conf_widget` actually creates `CONF_WIDGET`, a
// different (case-sensitive) object than the lowercase `"conf_widget"`
// every later query asks for via quoteIdent. Quoting every identifier in
// this DDL forces Oracle to store the exact lowercase name the rest of
// the adapter expects. The bare word `uid` also needed quoting for a
// second, unrelated reason even before this fix: Oracle reserves it as a
// pseudo-column (current session's numeric user ID), confirmed via a
// separate ORA-03050 error — quoting satisfies both requirements at once.
var oracleConformanceSchema = dialecttest.Schema{
	Widget: `CREATE TABLE "conf_widget" (
		"id" NUMBER(19) GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		"name" VARCHAR2(50) NOT NULL,
		"bio" CLOB,
		"category" VARCHAR2(50),
		"score" NUMBER(19),
		"price" NUMBER(10,2),
		"ratio" FLOAT,
		"active" NUMBER(1),
		"code" CHAR(8),
		"born" DATE,
		"seen" TIMESTAMP,
		"duration" TIMESTAMP,
		"data" BLOB,
		"uid" VARCHAR2(36),
		"meta" CLOB
	)`,
	Deleted: `CREATE TABLE "conf_deleted" (
		"id" NUMBER(19) GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		"name" VARCHAR2(50) NOT NULL,
		"deleted_at" TIMESTAMP
	)`,
	Parent: `CREATE TABLE "conf_parent" (
		"id" NUMBER(19) GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		"name" VARCHAR2(50) NOT NULL UNIQUE
	)`,
	CascadeChild: `CREATE TABLE "conf_cascade_child" (
		"id" NUMBER(19) GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		"parent_id" NUMBER(19) NOT NULL,
		"name" VARCHAR2(50) NOT NULL,
		FOREIGN KEY ("parent_id") REFERENCES "conf_parent"("id") ON DELETE CASCADE
	)`,
	SetNullChild: `CREATE TABLE "conf_setnull_child" (
		"id" NUMBER(19) GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		"parent_id" NUMBER(19),
		"name" VARCHAR2(50) NOT NULL,
		FOREIGN KEY ("parent_id") REFERENCES "conf_parent"("id") ON DELETE SET NULL
	)`,
	// Oracle's SQL grammar has no ON DELETE RESTRICT keyword at all (only
	// CASCADE/SET NULL, or omitting ON DELETE entirely) — omitting the
	// clause is Oracle's own default behavior and blocks the parent delete
	// when referencing child rows exist, the functional equivalent
	// golem's OnDeleteRestrict needs, same reasoning as driver/mssql's
	// NO ACTION choice.
	RestrictChild: `CREATE TABLE "conf_restrict_child" (
		"id" NUMBER(19) GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		"parent_id" NUMBER(19) NOT NULL,
		"name" VARCHAR2(50) NOT NULL,
		FOREIGN KEY ("parent_id") REFERENCES "conf_parent"("id")
	)`,
}

// oracleCapabilities: confirmed via design.md's probe — FOR UPDATE,
// NOWAIT, and SKIP LOCKED all compile and execute without error. Oracle
// has no FOR SHARE clause at all (a permanent SQL-dialect gap, not a
// version gate), and no NO KEY UPDATE/KEY SHARE equivalent (Postgres-
// specific), same stance as driver/mysql/driver/mssql for the latter two.
var oracleCapabilities = dialecttest.Capabilities{
	Locking: dialecttest.LockCapabilities{
		Update:      true,
		NoKeyUpdate: false,
		Share:       false,
		KeyShare:    false,
		NoWait:      true,
		SkipLocked:  true,
	},
}

func TestOracle_Conformance(t *testing.T) {
	dsn := testDSN()
	dialecttest.Run(t, oracleConformanceSchema, oracleCapabilities, New(func(o *Options) {
		o.DSN = dsn
	}))
}
