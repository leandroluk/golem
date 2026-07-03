package golem

// BIGINT is a 64-bit integer column type.
func BIGINT() ColumnType {
	return ColumnType{kind: "bigint"}
}

// VARCHAR is a variable-length string column type with a maximum length n.
func VARCHAR(n int) ColumnType {
	return ColumnType{kind: "varchar", length: n}
}

// TEXT is an unbounded-length string column type.
func TEXT() ColumnType {
	return ColumnType{kind: "text"}
}
