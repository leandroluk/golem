package postgres

import (
	"testing"

	"github.com/leandroluk/golem/internal/stmt"
)

func TestCompileSelect_Lock_ForUpdate(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Lock:  &stmt.LockClause{Strength: "update"},
	}
	sql, _, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT * FROM "users" FOR UPDATE`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
}

func TestCompileSelect_Lock_AllStrengths(t *testing.T) {
	cases := []struct {
		strength string
		want     string
	}{
		{"update", "FOR UPDATE"},
		{"no_key_update", "FOR NO KEY UPDATE"},
		{"share", "FOR SHARE"},
		{"key_share", "FOR KEY SHARE"},
	}
	for _, tc := range cases {
		t.Run(tc.strength, func(t *testing.T) {
			d := dialect{}
			s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: tc.strength}}
			sql, _, err := d.CompileSelect(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := `SELECT * FROM "users" ` + tc.want
			if sql != want {
				t.Errorf("sql = %q, want %q", sql, want)
			}
		})
	}
}

func TestCompileSelect_Lock_WaitPolicies(t *testing.T) {
	cases := []struct {
		wait string
		want string
	}{
		{"", `SELECT * FROM "users" FOR UPDATE`},
		{"nowait", `SELECT * FROM "users" FOR UPDATE NOWAIT`},
		{"skip_locked", `SELECT * FROM "users" FOR UPDATE SKIP LOCKED`},
	}
	for _, tc := range cases {
		t.Run(tc.wait, func(t *testing.T) {
			d := dialect{}
			s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "update", Wait: tc.wait}}
			sql, _, err := d.CompileSelect(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
		})
	}
}

func TestCompileSelect_Lock_UnsupportedStrength_ReturnsError(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "bogus"}}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error for unsupported lock strength, got nil")
	}
}

func TestCompileSelect_Lock_UnsupportedWait_ReturnsError(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "update", Wait: "bogus"}}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error for unsupported lock wait policy, got nil")
	}
}

func TestCompileSelect_Lock_WithOrderByLimitOffset_AppearsLast(t *testing.T) {
	d := dialect{}
	limit := 10
	s := &stmt.Select{
		Table:   "users",
		OrderBy: []stmt.OrderElement{{Column: "id", Desc: false}},
		Limit:   &limit,
		Lock:    &stmt.LockClause{Strength: "update"},
	}
	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT * FROM "users" ORDER BY "id" ASC LIMIT $1 FOR UPDATE`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != 10 {
		t.Fatalf("args = %v, want [10]", args)
	}
}
