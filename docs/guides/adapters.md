# Supported databases

Golem ships 5 production-grade adapters. Every one implements the same `golem.Dialect`/`golem.Connector` contract, so switching databases means changing one import (`driver/postgres` → `driver/mysql`, etc.) — entity declarations, `Repository[T]` calls, and query criteria never change.

The adapter set is deliberately capped at these 5 — the same scope philosophy mainstream ORMs like TypeORM follow, rather than chasing exhaustive database coverage. See the project's `.specs/project/STATE.md` for the reasoning behind two other databases (IBM Db2, Snowflake) that were evaluated and dropped.

## At a glance

|                           | Postgres           | MySQL / MariaDB                   | SQL Server                             | SQLite                      | Oracle                                 |
| ------------------------- | ------------------ | --------------------------------- | -------------------------------------- | --------------------------- | -------------------------------------- |
| Driver                    | `jackc/pgx/v5`     | `go-sql-driver/mysql`             | `microsoft/go-mssqldb`                 | `modernc.org/sqlite`        | `sijms/go-ora/v2`                      |
| Native dependency         | none               | none                              | none                                   | none (pure Go)              | none (pure Go)                         |
| Placeholders              | `$1, $2, ...`      | `?`                               | `@p1, @p2, ...`                        | `?`                         | `:1, :2, ...`                          |
| Identifier quoting        | `"double quotes"`  | `` `backticks` ``                 | `[brackets]`                           | `"double quotes"`           | `"double quotes"`                      |
| Unquoted identifier case  | lowercase          | lowercase                         | preserved                              | preserved                   | **UPPERCASE**                          |
| Pagination                | `LIMIT x OFFSET y` | `LIMIT x OFFSET y`                | `OFFSET y ROWS FETCH NEXT x ROWS ONLY` | `LIMIT x OFFSET y`          | `OFFSET y ROWS FETCH NEXT x ROWS ONLY` |
| Insert/Update round-trips | 1 (`RETURNING *`)  | 2 (`LAST_INSERT_ID()` + `SELECT`) | 1 (`OUTPUT INSERTED.*`)                | 1 (`RETURNING *`, 3.35+)    | 2 (write + `SELECT`)                   |
| Row locking               | full               | partial                           | partial (table hint)                   | none                        | partial                                |
| Minimum version           | any modern         | MySQL 8 / MariaDB best-effort     | 2019+                                  | any (3.35+ for `RETURNING`) | 12c+                                   |

See [Pessimistic locking](locking.md#support-matrix-per-adapter) for the exact lock-strength/wait-mode matrix, and [Typed errors](errors.md#what-each-adapter-maps) for error-code mapping.

## Oracle's identifier case-folding — the one real gotcha

Every adapter's `quoteIdent` always quotes identifiers, preserving the exact case declared in `golem.NewTable` — so this is invisible from `Repository[T]`'s side. It only matters if you write **raw SQL** (see [Raw SQL](raw-sql.md)) against Oracle without quoting your own identifiers: Oracle folds *unquoted* identifiers to **UPPERCASE**, the opposite of Postgres/MySQL/SQLite (lowercase) and the opposite of what every entity's compiled queries expect (which stay in whatever case the Go struct/column name declares, always quoted). A raw `SELECT * FROM users` against a table golem created as `"users"` (lowercase, quoted) will fail with `ORA-00942: table or view does not exist`, because unquoted `users` resolves to `USERS`, a different object. Quote your own raw SQL identifiers when targeting Oracle: `` `SELECT * FROM "users"` ``.

## Column type mapping

How each `golem.ColumnType` (see [Declaring schemas](schema.md#golemcolumntype)) maps to a real SQL type per dialect:

| `golem.ColumnType` | Postgres           | MySQL / MariaDB   | SQL Server         | SQLite    | Oracle                 |
| ------------------ | ------------------ | ----------------- | ------------------ | --------- | ---------------------- |
| `BOOLEAN()`        | `BOOLEAN`          | `TINYINT(1)`      | `BIT`              | `INTEGER` | `NUMBER(1)`            |
| `SMALLINT()`       | `SMALLINT`         | `SMALLINT`        | `SMALLINT`         | `INTEGER` | `NUMBER(5)`            |
| `INTEGER()`        | `INTEGER`          | `INT`             | `INT`              | `INTEGER` | `NUMBER(10)`           |
| `BIGINT()`         | `BIGINT`           | `BIGINT`          | `BIGINT`           | `INTEGER` | `NUMBER(19)`           |
| `DECIMAL(p,s)`     | `NUMERIC(p,s)`     | `DECIMAL(p,s)`    | `DECIMAL(p,s)`     | `REAL`    | `NUMBER(p,s)`          |
| `FLOAT()`          | `DOUBLE PRECISION` | `DOUBLE`          | `FLOAT`            | `REAL`    | `FLOAT`                |
| `CHAR(n)`          | `CHAR(n)`          | `CHAR(n)`         | `NCHAR(n)`         | `TEXT`    | `CHAR(n)`              |
| `VARCHAR(n)`       | `VARCHAR(n)`       | `VARCHAR(n)`      | `NVARCHAR(n)`      | `TEXT`    | `VARCHAR2(n)`          |
| `TEXT()`           | `TEXT`             | `TEXT`/`LONGTEXT` | `NVARCHAR(MAX)`    | `TEXT`    | `CLOB`                 |
| `DATE()`           | `DATE`             | `DATE`            | `DATE`             | `TEXT`    | `DATE`                 |
| `DATETIME()`       | `TIMESTAMP`        | `DATETIME`        | `DATETIME2`        | `TEXT`    | `TIMESTAMP`            |
| `TIME()`           | `TIME`             | `TIME`            | `TIME`             | `TEXT`    | `TIMESTAMP` (see note) |
| `BLOB()`           | `BYTEA`            | `BLOB`/`LONGBLOB` | `VARBINARY(MAX)`   | `BLOB`    | `BLOB`                 |
| `UUID()`           | `UUID` (native)    | `CHAR(36)`        | `UNIQUEIDENTIFIER` | `TEXT`    | `VARCHAR2(36)`         |
| `JSON()`           | `JSONB`            | `JSON`            | `NVARCHAR(MAX)`    | `TEXT`    | `CLOB`                 |

Notes:

- **`TIME()` on Oracle** maps to `TIMESTAMP`, not `INTERVAL DAY TO SECOND` — the driver can't bind a Go `time.Time` into an `INTERVAL` column, so `TIMESTAMP` is used at the cost of losing `INTERVAL`'s multi-day-duration range (a value class you almost never need for a plain "time of day" field).
- **`UUID()`**: only Postgres has a native `UUID` type. Every other adapter stores it as a 36-character hyphenated string (`VARCHAR(36)`/`CHAR(36)`), never a packed 16-byte binary form — this avoids any byte-order/endianness ambiguity when reading it back.
- **`golem.ColumnType` never becomes DDL.** These mappings only tell each adapter how to `Bind`/`Scan` a value correctly — golem doesn't generate `CREATE TABLE` statements. Your own migration tool needs to create columns using types compatible with this table.

## Upsert

None of the 5 adapters implement upsert semantics (`INSERT ... ON CONFLICT`/`ON DUPLICATE KEY UPDATE`/`MERGE`) — `SaveOne`/`SaveMany` always route through a normal `Update` by primary key, never a database-level upsert statement.

## Per-adapter connection details

See [Connecting](connecting.md) for each adapter's exact `Options` fields and example DSN.
