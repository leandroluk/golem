package sqlite

import "testing"

func TestNew(t *testing.T) {
	opt := New(func(o *Options) {
		o.Path = ":memory:"
	})
	if opt == nil {
		t.Fatal("expected option")
	}
}
