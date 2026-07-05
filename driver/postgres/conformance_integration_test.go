//go:build integration

package postgres

import (
	"testing"

	"github.com/leandroluk/golem/internal/dialecttest"
)

// postgresConformanceSchema supplies dialecttest's fixed logical tables as
// real Postgres DDL. Every statement uses CREATE TEMPORARY TABLE (session-
// scoped, no explicit teardown needed) and table names matching exactly
// what internal/dialecttest/entities.go declares via TableName(...).
var postgresConformanceSchema = dialecttest.Schema{
	// Only id/name are NOT NULL: golem's Insert omits any zero-value Go
	// field from the INSERT entirely (see repository.Insert), and not every
	// conformance subtest populates every column (BindScanRoundTrip sets
	// all of them; CRUD/Aggregates set name/category/score; Locking sets
	// only name) — every other column must tolerate being absent.
	Widget: `CREATE TEMPORARY TABLE conf_widget (
		id BIGSERIAL PRIMARY KEY,
		name VARCHAR(50) NOT NULL,
		bio TEXT,
		category VARCHAR(50),
		score BIGINT,
		price NUMERIC(10,2),
		ratio DOUBLE PRECISION,
		active BOOLEAN,
		code CHAR(8),
		born DATE,
		seen TIMESTAMP,
		duration TIME,
		data BYTEA,
		uid UUID,
		meta JSONB
	)`,
	Deleted: `CREATE TEMPORARY TABLE conf_deleted (
		id BIGSERIAL PRIMARY KEY,
		name VARCHAR(50) NOT NULL,
		deleted_at TIMESTAMP
	)`,
	Parent: `CREATE TEMPORARY TABLE conf_parent (
		id BIGSERIAL PRIMARY KEY,
		name VARCHAR(50) NOT NULL UNIQUE
	)`,
	CascadeChild: `CREATE TEMPORARY TABLE conf_cascade_child (
		id BIGSERIAL PRIMARY KEY,
		parent_id BIGINT NOT NULL REFERENCES conf_parent(id) ON DELETE CASCADE,
		name VARCHAR(50) NOT NULL
	)`,
	SetNullChild: `CREATE TEMPORARY TABLE conf_setnull_child (
		id BIGSERIAL PRIMARY KEY,
		parent_id BIGINT REFERENCES conf_parent(id) ON DELETE SET NULL,
		name VARCHAR(50) NOT NULL
	)`,
	RestrictChild: `CREATE TEMPORARY TABLE conf_restrict_child (
		id BIGSERIAL PRIMARY KEY,
		parent_id BIGINT NOT NULL REFERENCES conf_parent(id) ON DELETE RESTRICT,
		name VARCHAR(50) NOT NULL
	)`,
}

// postgresCapabilities: Postgres supports every lock strength/wait mode
// M14 built.
var postgresCapabilities = dialecttest.Capabilities{
	Locking: dialecttest.LockCapabilities{
		Update:      true,
		NoKeyUpdate: true,
		Share:       true,
		KeyShare:    true,
		NoWait:      true,
		SkipLocked:  true,
	},
}

func TestPostgres_Conformance(t *testing.T) {
	dsn := testDSN()
	dialecttest.Run(t, postgresConformanceSchema, postgresCapabilities, New(func(o *Options) {
		o.DSN = dsn
	}))
}
