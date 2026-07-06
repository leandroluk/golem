package oracle

import "testing"

func TestNew(t *testing.T) {
	opt := New(func(o *Options) {
		o.DSN = "dummy"
	})
	if opt == nil {
		t.Fatal("expected option")
	}
}
