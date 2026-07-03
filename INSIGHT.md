### Tipos a serem criados no projeto para atender a maioria dos bancos de dados sql

| **Nome Genérico** | **PostgreSQL**   | **MySQL / MariaDB**  | **SQL Server (MSSQL)** | **Oracle**             | **SQLite** | **IBM Db2**           | **Snowflake (OLAP)** |
| ----------------- | ---------------- | -------------------- | ---------------------- | ---------------------- | ---------- | --------------------- | -------------------- |
| **BOOLEAN**       | BOOLEAN          | TINYINT(1) / BOOLEAN | BIT                    | NUMBER(1)              | INTEGER    | BOOLEAN (v11.1+)      | BOOLEAN              |
| **SMALLINT**      | SMALLINT         | SMALLINT             | SMALLINT               | NUMBER(5)              | INTEGER    | SMALLINT              | SMALLINT             |
| **INTEGER**       | INTEGER          | INT                  | INT                    | NUMBER(10)             | INTEGER    | INTEGER               | INTEGER              |
| **BIGINT**        | BIGINT           | BIGINT               | BIGINT                 | NUMBER(19)             | INTEGER    | BIGINT                | BIGINT               |
| **DECIMAL(p,s)**  | NUMERIC(p,s)     | DECIMAL(p,s)         | DECIMAL(p,s)           | NUMBER(p,s)            | REAL       | DECIMAL(p,s)          | NUMBER(p,s)          |
| **FLOAT**         | DOUBLE PRECISION | DOUBLE               | FLOAT                  | FLOAT                  | REAL       | DOUBLE                | FLOAT                |
| **CHAR(n)**       | CHAR(n)          | CHAR(n)              | NCHAR(n)               | CHAR(n)                | TEXT       | CHAR(n)               | CHAR(n)              |
| **VARCHAR(n)**    | VARCHAR(n)       | VARCHAR(n)           | NVARCHAR(n)            | VARCHAR2(n)            | TEXT       | VARCHAR(n)            | VARCHAR(n)           |
| **TEXT**          | TEXT             | TEXT / LONGTEXT      | NVARCHAR(MAX)          | CLOB                   | TEXT       | CLOB                  | VARCHAR / TEXT       |
| **DATE**          | DATE             | DATE                 | DATE                   | DATE                   | TEXT       | DATE                  | DATE                 |
| **DATETIME**      | TIMESTAMP        | DATETIME             | DATETIME2              | TIMESTAMP              | TEXT       | TIMESTAMP             | TIMESTAMP_NTZ        |
| **TIME**          | TIME             | TIME                 | TIME                   | INTERVAL DAY TO SECOND | TEXT       | TIME                  | TIME                 |
| **BLOB**          | BYTEA            | BLOB / LONGBLOB      | VARBINARY(MAX)         | BLOB                   | BLOB       | BLOB                  | BINARY               |
| **UUID**          | UUID             | CHAR(36)             | UNIQUEIDENTIFIER       | RAW(16) / VARCHAR2(36) | TEXT       | CHAR(16) FOR BIT DATA | VARCHAR(36)          |
| **JSON**          | JSONB            | JSON                 | NVARCHAR(MAX)          | CLOB / JSON (21c+)     | TEXT       | BSON / CLOB           | VARIANT              |

---

### Equivalências declarativas entre tipos de bancos

| **Nome Genérico**  | **PostgreSQL**               | **MySQL / MariaDB**       | **SQL Server (MSSQL)**    | **Oracle**                   | **SQLite**                | **IBM Db2**                  | **Snowflake (OLAP)**                       |
| ------------------ | ---------------------------- | ------------------------- | ------------------------- | ---------------------------- | ------------------------- | ---------------------------- | ------------------------------------------ |
| **PRIMARY_KEY**    | PRIMARY KEY                  | PRIMARY KEY               | PRIMARY KEY               | PRIMARY KEY                  | PRIMARY KEY               | PRIMARY KEY                  | PRIMARY KEY                                |
| **AUTO_INCREMENT** | GENERATED ALWAYS AS IDENTITY | AUTO_INCREMENT            | IDENTITY(1,1)             | GENERATED ALWAYS AS IDENTITY | AUTOINCREMENT             | GENERATED ALWAYS AS IDENTITY | AUTOINCREMENT                              |
| **NOT_NULL**       | NOT NULL                     | NOT NULL                  | NOT NULL                  | NOT NULL                     | NOT NULL                  | NOT NULL                     | NOT NULL                                   |
| **NULLABLE**       | NULL                         | NULL                      | NULL                      | NULL                         | NULL                      | NULL                         | NULL                                       |
| **UNIQUE**         | UNIQUE                       | UNIQUE                    | UNIQUE                    | UNIQUE                       | UNIQUE                    | UNIQUE                       | UNIQUE                                     |
| **DEFAULT**        | DEFAULT x                    | DEFAULT x                 | DEFAULT x                 | DEFAULT x                    | DEFAULT x                 | DEFAULT x                    | DEFAULT x                                  |
| **DEFAULT_NOW**    | DEFAULT CURRENT_TIMESTAMP    | DEFAULT CURRENT_TIMESTAMP | DEFAULT CURRENT_TIMESTAMP | DEFAULT CURRENT_TIMESTAMP    | DEFAULT CURRENT_TIMESTAMP | DEFAULT CURRENT_TIMESTAMP    | DEFAULT CURRENT_TIMESTAMP                  |
| **FOREIGN_KEY**    | REFERENCES table(id)         | REFERENCES table(id)      | REFERENCES table(id)      | REFERENCES table(id)         | REFERENCES table(id)      | REFERENCES table(id)         | REFERENCES table(id)                       |
| **CHECK**          | CHECK(cond)                  | CHECK(cond)               | CHECK(cond)               | CHECK(cond)                  | CHECK(cond)               | CHECK(cond)                  | <span style="color:red">Unsupported</span> |

---

### Equivalências de Paginação (DQL)

| **Nome Genérico** | **PostgreSQL**   | **MySQL / MariaDB** | **SQL Server (MSSQL)**               | **Oracle**                                  | **SQLite**       | **IBM Db2**                          | **Snowflake (OLAP)** |
| ----------------- | ---------------- | ------------------- | ------------------------------------ | ------------------------------------------- | ---------------- | ------------------------------------ | -------------------- |
| **PAGINATION**    | LIMIT x OFFSET y | LIMIT x OFFSET y    | OFFSET y ROWS FETCH NEXT x ROWS ONLY | OFFSET y ROWS FETCH NEXT x ROWS ONLY (12c+) | LIMIT x OFFSET y | OFFSET y ROWS FETCH NEXT x ROWS ONLY | LIMIT x OFFSET y     |

---

### Equivalências de Índices Avançados

| **Nome Genérico** | **PostgreSQL**           | **MySQL / MariaDB** | **SQL Server (MSSQL)** | **Oracle**           | **SQLite**               | **IBM Db2**       | **Snowflake (OLAP)** |
| ----------------- | ------------------------ | ------------------- | ---------------------- | -------------------- | ------------------------ | ----------------- | -------------------- |
| **INDEX**         | CREATE INDEX             | CREATE INDEX        | CREATE INDEX           | CREATE INDEX         | CREATE INDEX             | CREATE INDEX      | CREATE INDEX         |
| **JSON_INDEX**    | CREATE INDEX...USING GIN | Multi-Valued Index  | Computed Column Index  | JSON Search Index    | CREATE INDEX (Expressão) | N/A (JSON nativo) | CREATE INDEX         |
| **TEXT_INDEX**    | GIN / GiST               | FULLTEXT INDEX      | FULLTEXT CATALOG       | Oracle Text (CTXSYS) | FTS5 (Virtual Table)     | N/A (JSON nativo) | CREATE INDEX (TEXT)  |

---

### Equivalências de UPSERT (Inserção Condicional)

| **Nome Genérico** | **PostgreSQL**                             | **MySQL / MariaDB**                | **SQL Server (MSSQL)**                     | **Oracle**                                 | **SQLite**                                 | **IBM Db2**                                | **Snowflake (OLAP)**                       |
| ----------------- | ------------------------------------------ | ---------------------------------- | ------------------------------------------ | ------------------------------------------ | ------------------------------------------ | ------------------------------------------ | ------------------------------------------ |
| **UPSERT**        | INSERT ... ON CONFLICT (key) DO UPDATE SET | INSERT ... ON DUPLICATE KEY UPDATE | MERGE INTO target USING source ON cond ... | MERGE INTO target USING source ON cond ... | INSERT ... ON CONFLICT (key) DO UPDATE SET | MERGE INTO target USING source ON cond ... | MERGE INTO target USING source ON cond ... |

---

### Equivalências de Transações

| **Nome Genérico** | **PostgreSQL** | **MySQL / MariaDB** | **SQL Server (MSSQL)** | **Oracle** | **SQLite** | **IBM Db2** | **Snowflake (OLAP)** |
| ----------------- | -------------- | ------------------- | ---------------------- | ---------- | ---------- | ----------- | -------------------- |
| **START**         | BEGIN          | START TRANSACTION   | BEGIN TRANSACTION      | BEGIN      | BEGIN      | BEGIN       | BEGIN TRANSACTION    |
| **COMMIT**        | COMMIT         | COMMIT              | COMMIT                 | COMMIT     | COMMIT     | COMMIT      | COMMIT               |
| **ROLLBACK**      | ROLLBACK       | ROLLBACK            | ROLLBACK               | ROLLBACK   | ROLLBACK   | ROLLBACK    | ROLLBACK             |

---

### Equivalências de Operadores e Expressões

| **Nome Genérico**        | **PostgreSQL**       | **MySQL / MariaDB**           | **SQL Server (MSSQL)** | **Oracle**             | **SQLite**          | **IBM Db2**            | **Snowflake (OLAP)** |
| ------------------------ | -------------------- | ----------------------------- | ---------------------- | ---------------------- | ------------------- | ---------------------- | -------------------- |
| **MODULO (Resto)**       | %                    | "% ou MOD(x,y)"               | %                      | "MOD(x, y)"            | %                   | "MOD(x, y)"            | %                    |
| **REGEX**                | ~                    | REGEXP                        | (Não nativo)           | REGEXP_LIKE()          | REGEXP              | REGEXP_LIKE()          | REGEXP / RLIKE       |
| **ILIKE (Case Insens.)** | ILIKE                | LIKE (1)                      | LIKE (1)               | UPPER(x) LIKE UPPER(y) | LIKE (1)            | UPPER(x) LIKE UPPER(y) | ILIKE                |
| **DATE_ADD (n, unit)**   | x + INTERVAL '1 day' | "DATE_ADD(x, INTERVAL 1 DAY)" | "DATEADD(day, 1, x)"   | x + 1                  | "DATE(x, '+1 day')" | x + 1 DAY              | "DATEADD(day, 1, x)" |
| **BITWISE_AND**          | &                    | &                             | &                      | "BITAND(x, y)"         | &                   | "BITAND(x, y)"         | "BITAND(x, y)"       |
| **BITWISE_XOR**          | ###                  | ^                             | ^                      | (Não nativo)           | ~                   | "BITXOR(x, y)"         | "BITXOR(x, y)"       |
| **IS(cond)**             | x IS TRUE            | x IS TRUE / x = 1             | x = 1                  | x = 1                  | x = 1               | x = 1                  | x IS TRUE            |
| **IS_NULL**              | IS NULL              | IS NULL                       | IS NULL                | IS NULL                | IS NULL             | IS NULL                | IS NULL              |

---

### Equivalências para resolução de consultas que contém FullTextSearch

#### Nome Genérico

```sql
SELECT * FROM "table" 
WHERE SEARCH("column_name", 'search_term', 2 /* levenshtein size */)
```

#### PostgreSQL

```sql
USING arguments AS SELECT "column_name" AS column_name, 'search_term' AS search_term, 2 AS levenshtein_size
SELECT * FROM "table", arguments
WHERE (
  /* similarity            */ column_name %% search_term OR
  /* soundex               */ soundex(column_name) = soundex(search_term) OR
  /* levenshtein           */ levenshtein(lower(column_name), lower(search_term)) <= levenshtein_size OR
  /* case insensitive like */ column_name ilike '%' || search_term || '%' 
)
```

#### MySQL / MariaDB

```sql
WITH arguments AS SELECT 'search_term' AS search_term, 2 AS levenshtein_size
SELECT t.* FROM "table" t
JOIN arguments a
WHERE (
  /* soundex               */ SOUNDEX(t."column_name") = SOUNDEX(a.search_term) OR
  /* levenshtein           */ LEVENSHTEIN(LOWER(t."column_name"), LOWER(a.search_term)) <= a.levenshtein_size OR
  /* case insensitive like */ t."column_name" LIKE CONCAT('%', a.search_term, '%')
)
```

#### SQL Server (MSSQL)

```sql
WITH arguments AS SELECT 'search_term' AS search_term, 2 AS levenshtein_size
SELECT t.* FROM "table" t
CROSS JOIN arguments a
WHERE (
  /* difference            */ DIFFERENCE(t."column_name", a.search_term) >= 3 OR
  /* soundex               */ SOUNDEX(t."column_name") = SOUNDEX(a.search_term) OR
  /* levenshtein           */ dbo.LEVENSHTEIN(LOWER(t."column_name"), LOWER(a.search_term)) <= a.levenshtein_size OR
  /* case insensitive like */ t."column_name" LIKE '%' + a.search_term + '%'
)
```

#### Oracle

```sql
WITH arguments AS SELECT 'search_term' AS search_term, 2 AS levenshtein_size FROM DUAL
SELECT t.* FROM "table" t
CROSS JOIN arguments a
WHERE (
  /* similarity            */ UTL_MATCH.JARO_WINKLER_SIMILARITY(t."column_name", a.search_term) >= 80 OR
  /* soundex               */ SOUNDEX(t."column_name") = SOUNDEX(a.search_term) OR
  /* levenshtein           */ UTL_MATCH.EDIT_DISTANCE(LOWER(t."column_name"), LOWER(a.search_term)) <= a.levenshtein_size OR
  /* case insensitive like */ UPPER(t."column_name") LIKE UPPER('%' || a.search_term || '%')
)
```

#### SQLite

```sql
WITH arguments AS SELECT 'search_term' AS search_term, 2 AS levenshtein_size
SELECT t.* FROM "table" t, arguments a
WHERE (
  /* levenshtein           */ LEVENSHTEIN(LOWER(t."column_name"), LOWER(a.search_term)) <= a.levenshtein_size OR
  /* case insensitive like */ t."column_name" LIKE '%' || a.search_term || '%'
)
```

#### IBM Db2

```sql
WITH arguments AS SELECT 'search_term' AS search_term, 2 AS levenshtein_size FROM SYSIBM.SYSDUMMY1
SELECT t.* FROM "table" t
CROSS JOIN arguments a
WHERE (
  /* soundex               */ SOUNDEX(t."column_name") = SOUNDEX(a.search_term) OR
  /* case insensitive like */ UPPER(t."column_name") LIKE UPPER('%' || a.search_term || '%')
)
```

#### Snowflake (OLAP)

```sql
WITH arguments AS SELECT 'search_term' AS search_term, 2 AS levenshtein_size
SELECT t.* FROM "table" t
CROSS JOIN arguments a
WHERE (
  /* similarity            */ JAROWINKLER_SIMILARITY(t."column_name", a.search_term) >= 80 OR
  /* soundex               */ SOUNDEX(t."column_name") = SOUNDEX(a.search_term) OR
  /* levenshtein           */ EDITDISTANCE(LOWER(t."column_name"), LOWER(a.search_term)) <= a.levenshtein_size OR
  /* case insensitive like */ t."column_name" ILIKE '%' || a.search_term || '%'
)
```
