//go:build integration

package mysql

import (
	"testing"

	"github.com/leandroluk/golem/internal/dialecttest"
)

// mysqlConformanceSchema supplies dialecttest's fixed logical tables as
// real MySQL DDL. Unlike driver/postgres's schema, these use CREATE TABLE
// IF NOT EXISTS rather than CREATE TEMPORARY TABLE: MySQL does not support
// FOREIGN KEY constraints on temporary tables at all (needed here so
// ConflictDetection's ErrForeignKeyViolation case has a real constraint to
// violate), and the integration-test MySQL container is recreated fresh by
// `task test-integration` on every run anyway, so no cross-run cleanup is
// actually needed in practice.
var mysqlConformanceSchema = dialecttest.Schema{
	Widget: `CREATE TABLE IF NOT EXISTS conf_widget (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(50) NOT NULL,
		bio TEXT,
		category VARCHAR(50),
		score BIGINT,
		price DECIMAL(10,2),
		ratio DOUBLE,
		active BOOLEAN,
		code CHAR(8),
		born DATE,
		seen DATETIME,
		duration TIME,
		data BLOB,
		uid CHAR(36),
		meta JSON
	)`,
	Deleted: `CREATE TABLE IF NOT EXISTS conf_deleted (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(50) NOT NULL,
		deleted_at DATETIME
	)`,
	Parent: `CREATE TABLE IF NOT EXISTS conf_parent (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(50) NOT NULL UNIQUE
	)`,
	CascadeChild: `CREATE TABLE IF NOT EXISTS conf_cascade_child (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		parent_id BIGINT NOT NULL,
		name VARCHAR(50) NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES conf_parent(id) ON DELETE CASCADE
	)`,
	SetNullChild: `CREATE TABLE IF NOT EXISTS conf_setnull_child (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		parent_id BIGINT,
		name VARCHAR(50) NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES conf_parent(id) ON DELETE SET NULL
	)`,
	RestrictChild: `CREATE TABLE IF NOT EXISTS conf_restrict_child (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		parent_id BIGINT NOT NULL,
		name VARCHAR(50) NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES conf_parent(id) ON DELETE RESTRICT
	)`,
}

// mysqlCapabilities: MySQL 8+ has no NO KEY UPDATE/KEY SHARE equivalent
// (Postgres-specific) — every other lock strength/wait mode is supported.
var mysqlCapabilities = dialecttest.Capabilities{
	Locking: dialecttest.LockCapabilities{
		Update:      true,
		NoKeyUpdate: false,
		Share:       true,
		KeyShare:    false,
		NoWait:      true,
		SkipLocked:  true,
	},
}

func TestMySQL_Conformance(t *testing.T) {
	dsn := testDSN()
	dialecttest.Run(t, mysqlConformanceSchema, mysqlCapabilities, New(func(o *Options) {
		o.DSN = dsn
	}))
}
