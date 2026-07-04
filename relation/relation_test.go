package relation

import "testing"

func TestNewForeignKeyOptions_Defaults(t *testing.T) {
	o := NewForeignKeyOptions()
	if o.ResolvedOnDelete() != OnDeleteDefault {
		t.Errorf("OnDelete default = %q, want %q", o.ResolvedOnDelete(), OnDeleteDefault)
	}
}

func TestForeignKeyOptions_OnDelete(t *testing.T) {
	o := NewForeignKeyOptions().OnDelete(OnDeleteCascade)
	if o.ResolvedOnDelete() != OnDeleteCascade {
		t.Errorf("OnDelete = %q, want %q", o.ResolvedOnDelete(), OnDeleteCascade)
	}
}

func TestForeignKeyOptions_ChainReturnsSameInstance(t *testing.T) {
	o := NewForeignKeyOptions()
	got := o.OnDelete(OnDeleteRestrict)
	if got != o {
		t.Error("OnDelete should return the same *ForeignKeyOptions instance")
	}
}
