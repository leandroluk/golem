package sqlite

import (
	"testing"

	"github.com/leandroluk/golem/internal/dialecttest"
)

// sqliteConformanceSchema supplies dialecttest's fixed logical tables as
// real SQLite DDL. Plain CREATE TABLE (not CREATE TEMPORARY TABLE) is used:
// this adapter's connection pool is already bounded to 1 connection per
// dialecttest.Run call, against its own fresh :memory: database — that's
// already exactly as ephemeral as a temporary table would be, without
// needing the TEMPORARY keyword. INTEGER PRIMARY KEY is SQLite's
// autoincrement-eligible rowid alias; foreign key enforcement is only real
// because resolveDSN forces PRAGMA foreign_keys = ON for every connection.
//
// born/seen/duration/deleted_at are declared DATE/DATETIME/DATETIME rather
// than generic TEXT: SQLite stores them as TEXT either way (no native
// date/time storage class exists — see INSIGHT.md), but collectRows's
// normalizeCell needs the column's DECLARED type name (DatabaseTypeName())
// to tell a formatted date/time string apart from a plain TEXT column —
// confirmed empirically that SQLite's declared-type-name mechanism
// preserves exactly the keyword written here.
//
// No //go:build integration tag: unlike driver/postgres/driver/mysql, this
// suite never opens a real network connection (per TESTING.md's own rule of
// thumb — "anything that opens a real network connection is integration;
// anything else is unit") — an in-memory SQLite database is as fast and
// side-effect-free as any other unit test, so it belongs in the default
// `go test ./... -short` / `task coverage` run, not gated behind Docker's
// build tag. This is also what gives internal/dialecttest real (non-zero)
// coverage under `task coverage` for the first time — every prior adapter's
// conformance suite genuinely needs Docker, so internal/dialecttest was
// only ever exercised via `-tags=integration` before this.
var sqliteConformanceSchema = dialecttest.Schema{
	Widget: `CREATE TABLE conf_widget (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		bio TEXT,
		category TEXT,
		score INTEGER,
		price REAL,
		ratio REAL,
		active INTEGER,
		code TEXT,
		born DATE,
		seen DATETIME,
		duration TIME,
		data BLOB,
		uid TEXT,
		meta TEXT
	)`,
	Deleted: `CREATE TABLE conf_deleted (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		deleted_at DATETIME
	)`,
	Parent: `CREATE TABLE conf_parent (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL UNIQUE
	)`,
	CascadeChild: `CREATE TABLE conf_cascade_child (
		id INTEGER PRIMARY KEY,
		parent_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES conf_parent(id) ON DELETE CASCADE
	)`,
	SetNullChild: `CREATE TABLE conf_setnull_child (
		id INTEGER PRIMARY KEY,
		parent_id INTEGER,
		name TEXT NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES conf_parent(id) ON DELETE SET NULL
	)`,
	RestrictChild: `CREATE TABLE conf_restrict_child (
		id INTEGER PRIMARY KEY,
		parent_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES conf_parent(id) ON DELETE RESTRICT
	)`,
}

// sqliteCapabilities: SQLite has no row-level locking clause of any kind
// (it locks at the whole-database-file level instead) — every strength and
// wait mode is unconditionally unsupported. First adapter where
// Capabilities.Locking is entirely false.
var sqliteCapabilities = dialecttest.Capabilities{
	Locking: dialecttest.LockCapabilities{
		Update:      false,
		NoKeyUpdate: false,
		Share:       false,
		KeyShare:    false,
		NoWait:      false,
		SkipLocked:  false,
	},
}

func TestSQLite_Conformance(t *testing.T) {
	dialecttest.Run(t, sqliteConformanceSchema, sqliteCapabilities, New(func(o *Options) {
		o.Path = ":memory:"
	}))
}
