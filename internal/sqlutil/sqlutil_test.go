package sqlutil

import "testing"

func TestIsRowReturning(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"SELECT * FROM users", true},
		{"  select id from t", true},
		{"WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"SHOW TABLES", true},
		{"EXPLAIN SELECT 1", true},
		{"DESCRIBE users", true},
		{"DESC users", true},
		{"PRAGMA foreign_keys", true},
		{"(SELECT 1) UNION (SELECT 2)", true},
		{"INSERT INTO users (name) VALUES ('a')", false},
		{"UPDATE users SET name = 'a' WHERE id = 1", false},
		{"DELETE FROM users WHERE id = 1", false},
		{"CREATE TABLE t (id INT)", false},
		{"DROP TABLE t", false},
		{"", false},
		{"   ", false},
	}
	for _, tc := range cases {
		if got := IsRowReturning(tc.sql); got != tc.want {
			t.Errorf("IsRowReturning(%q) = %v, want %v", tc.sql, got, tc.want)
		}
	}
}
