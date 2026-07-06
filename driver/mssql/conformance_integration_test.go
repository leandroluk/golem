//go:build integration

package mssql

import (
	"testing"

	"github.com/leandroluk/golem/internal/dialecttest"
)

// mssqlConformanceSchema supplies dialecttest's fixed logical tables as
// real SQL Server DDL, per INSIGHT.md's type table (BOOLEAN->BIT, UUID->
// UNIQUEIDENTIFIER, CHAR/VARCHAR/TEXT/JSON->NCHAR/NVARCHAR/NVARCHAR(MAX),
// BLOB->VARBINARY(MAX), DATETIME->DATETIME2). IDENTITY(1,1) is SQL
// Server's auto-increment primary key. The integration-test SQL Server
// container is recreated fresh by `task test-integration` on every run, so
// plain CREATE TABLE (no cross-run cleanup) is fine, same reasoning as
// driver/mysql's own conformance schema.
var mssqlConformanceSchema = dialecttest.Schema{
	Widget: `CREATE TABLE conf_widget (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		name NVARCHAR(50) NOT NULL,
		bio NVARCHAR(MAX),
		category NVARCHAR(50),
		score BIGINT,
		price DECIMAL(10,2),
		ratio FLOAT,
		active BIT,
		code NCHAR(8),
		born DATE,
		seen DATETIME2,
		duration TIME,
		data VARBINARY(MAX),
		uid UNIQUEIDENTIFIER,
		meta NVARCHAR(MAX)
	)`,
	Deleted: `CREATE TABLE conf_deleted (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		name NVARCHAR(50) NOT NULL,
		deleted_at DATETIME2
	)`,
	Parent: `CREATE TABLE conf_parent (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		name NVARCHAR(50) NOT NULL UNIQUE
	)`,
	CascadeChild: `CREATE TABLE conf_cascade_child (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		parent_id BIGINT NOT NULL,
		name NVARCHAR(50) NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES conf_parent(id) ON DELETE CASCADE
	)`,
	SetNullChild: `CREATE TABLE conf_setnull_child (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		parent_id BIGINT,
		name NVARCHAR(50) NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES conf_parent(id) ON DELETE SET NULL
	)`,
	// SQL Server's T-SQL grammar has no ON DELETE RESTRICT keyword at all
	// (only CASCADE/NO ACTION/SET NULL/SET DEFAULT) — NO ACTION is the
	// functional equivalent golem's OnDeleteRestrict needs: it blocks the
	// parent delete when referencing child rows exist, same outcome as
	// every other adapter's RESTRICT.
	RestrictChild: `CREATE TABLE conf_restrict_child (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		parent_id BIGINT NOT NULL,
		name NVARCHAR(50) NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES conf_parent(id) ON DELETE NO ACTION
	)`,
}

// mssqlCapabilities: confirmed via design.md's probe — UPDLOCK/ROWLOCK,
// HOLDLOCK/ROWLOCK, NOWAIT, and READPAST table hints all compile and
// execute without error. NO KEY UPDATE/KEY SHARE have no SQL Server
// equivalent (Postgres-specific), same stance as driver/mysql.
var mssqlCapabilities = dialecttest.Capabilities{
	Locking: dialecttest.LockCapabilities{
		Update:      true,
		NoKeyUpdate: false,
		Share:       true,
		KeyShare:    false,
		NoWait:      true,
		SkipLocked:  true,
	},
}

func TestMSSQL_Conformance(t *testing.T) {
	dsn := testDSN()
	dialecttest.Run(t, mssqlConformanceSchema, mssqlCapabilities, New(func(o *Options) {
		o.DSN = dsn
	}))
}
