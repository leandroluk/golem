package golem

// BOOLEAN is a boolean column type.
func BOOLEAN() ColumnType {
	return ColumnType{kind: "boolean"}
}

// SMALLINT is a 64-bit integer column type.
func SMALLINT() ColumnType {
	return ColumnType{kind: "smallint"}
}

// INTEGER is a 32-bit integer column type.
func INTEGER() ColumnType {
	return ColumnType{kind: "integer"}
}

// BIGINT is a 64-bit integer column type.
func BIGINT() ColumnType {
	return ColumnType{kind: "bigint"}
}

// DECIMAL is a 64-bit integer column type.
func DECIMAL(precision, scale int) ColumnType {
	return ColumnType{kind: "decimal", precision: precision, scale: scale}
}

// FLOAT is a 64-bit integer column type.
func FLOAT(precision int) ColumnType {
	return ColumnType{kind: "float", precision: precision}
}

// CHAR is a variable-length string column type with a maximum length n.
func CHAR(length int) ColumnType {
	return ColumnType{kind: "char", length: length}
}

// VARCHAR is a variable-length string column type with a maximum length n.
func VARCHAR(length int) ColumnType {
	return ColumnType{kind: "varchar", length: length}
}

// TEXT is an unbounded-length string column type.
func TEXT() ColumnType {
	return ColumnType{kind: "text"}
}

// DATE is a date column type.
func DATE() ColumnType {
	return ColumnType{kind: "date"}
}

// DATETIME is a timezone-aware timestamp column type.
func DATETIME() ColumnType {
	return ColumnType{kind: "datetime"}
}

// TIME is a timestamp column type.
func TIME() ColumnType {
	return ColumnType{kind: "time"}
}

// BLOB is a BLOB column type.
func BLOB() ColumnType {
	return ColumnType{kind: "blob"}
}

// UUID is a UUID column type.
func UUID() ColumnType {
	return ColumnType{kind: "uuid"}
}

// JSON is a JSON/JSONB column type.
func JSON() ColumnType {
	return ColumnType{kind: "json"}
}

