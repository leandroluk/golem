package relation

import "testing"

func TestNewForeignKeyOptions_Defaults(t *testing.T) {
	o := NewForeignKeyOptions()
	if !o.ResolvedCreateForeignKeyConstraints() {
		t.Error("CreateForeignKeyConstraints default = false, want true")
	}
	if !o.ResolvedPersistence() {
		t.Error("Persistence default = false, want true")
	}
	if o.ResolvedOnDelete() != OnDeleteDefault {
		t.Errorf("OnDelete default = %q, want %q", o.ResolvedOnDelete(), OnDeleteDefault)
	}
	if o.ResolvedOnUpdate() != OnUpdateDefault {
		t.Errorf("OnUpdate default = %q, want %q", o.ResolvedOnUpdate(), OnUpdateDefault)
	}
	if o.ResolvedDeferrable() != DeferrableDefault {
		t.Errorf("Deferrable default = %q, want %q", o.ResolvedDeferrable(), DeferrableDefault)
	}
	if o.ResolvedLazy() {
		t.Error("Lazy default = true, want false")
	}
	if o.ResolvedEager() {
		t.Error("Eager default = true, want false")
	}
	if o.ResolvedOrphanedRowAction() != "" {
		t.Errorf("OrphanedRowAction default = %q, want empty", o.ResolvedOrphanedRowAction())
	}
	if o.HasCascade(CascadeInsert) {
		t.Error("HasCascade(Insert) default = true, want false")
	}
}

func TestForeignKeyOptions_OnDelete(t *testing.T) {
	o := NewForeignKeyOptions().OnDelete(OnDeleteCascade)
	if o.ResolvedOnDelete() != OnDeleteCascade {
		t.Errorf("OnDelete = %q, want %q", o.ResolvedOnDelete(), OnDeleteCascade)
	}
}

func TestForeignKeyOptions_OnUpdate(t *testing.T) {
	o := NewForeignKeyOptions().OnUpdate(OnUpdateSetNull)
	if o.ResolvedOnUpdate() != OnUpdateSetNull {
		t.Errorf("OnUpdate = %q, want %q", o.ResolvedOnUpdate(), OnUpdateSetNull)
	}
}

func TestForeignKeyOptions_Deferrable(t *testing.T) {
	o := NewForeignKeyOptions().Deferrable(DeferrableDeferred)
	if o.ResolvedDeferrable() != DeferrableDeferred {
		t.Errorf("Deferrable = %q, want %q", o.ResolvedDeferrable(), DeferrableDeferred)
	}
}

func TestForeignKeyOptions_CreateForeignKeyConstraints(t *testing.T) {
	o := NewForeignKeyOptions().CreateForeignKeyConstraints(false)
	if o.ResolvedCreateForeignKeyConstraints() {
		t.Error("CreateForeignKeyConstraints = true, want false after explicit false")
	}
}

func TestForeignKeyOptions_Lazy(t *testing.T) {
	o := NewForeignKeyOptions().Lazy(true)
	if !o.ResolvedLazy() {
		t.Error("Lazy = false, want true")
	}
}

func TestForeignKeyOptions_Eager(t *testing.T) {
	o := NewForeignKeyOptions().Eager(true)
	if !o.ResolvedEager() {
		t.Error("Eager = false, want true")
	}
}

func TestForeignKeyOptions_Persistence(t *testing.T) {
	o := NewForeignKeyOptions().Persistence(false)
	if o.ResolvedPersistence() {
		t.Error("Persistence = true, want false after explicit false")
	}
}

func TestForeignKeyOptions_OrphanedRowAction(t *testing.T) {
	o := NewForeignKeyOptions().OrphanedRowAction(OrphanedRowActionDelete)
	if o.ResolvedOrphanedRowAction() != OrphanedRowActionDelete {
		t.Errorf("OrphanedRowAction = %q, want %q", o.ResolvedOrphanedRowAction(), OrphanedRowActionDelete)
	}
}

func TestForeignKeyOptions_Cascade_IndividualFlags(t *testing.T) {
	o := NewForeignKeyOptions().Cascade(CascadeInsert, CascadeUpdate)
	if !o.HasCascade(CascadeInsert) {
		t.Error("HasCascade(Insert) = false, want true")
	}
	if !o.HasCascade(CascadeUpdate) {
		t.Error("HasCascade(Update) = false, want true")
	}
	if o.HasCascade(CascadeRemove) {
		t.Error("HasCascade(Remove) = true, want false (not set)")
	}
}

func TestForeignKeyOptions_Cascade_All(t *testing.T) {
	o := NewForeignKeyOptions().Cascade(CascadeAll)
	for _, opt := range []CascadeOption{CascadeInsert, CascadeUpdate, CascadeRemove, CascadeSoftRemove, CascadeRecover} {
		if !o.HasCascade(opt) {
			t.Errorf("HasCascade(%v) = false after Cascade(CascadeAll), want true", opt)
		}
	}
}

func TestForeignKeyOptions_Cascade_AccumulatesAcrossCalls(t *testing.T) {
	o := NewForeignKeyOptions()
	o.Cascade(CascadeInsert)
	o.Cascade(CascadeUpdate)
	if !o.HasCascade(CascadeInsert) || !o.HasCascade(CascadeUpdate) {
		t.Error("expected both CascadeInsert and CascadeUpdate set across separate calls")
	}
}

func TestForeignKeyOptions_ChainReturnsSameInstance(t *testing.T) {
	o := NewForeignKeyOptions()
	got := o.OnDelete(OnDeleteRestrict).OnUpdate(OnUpdateRestrict).Deferrable(DeferrableImmediate).
		CreateForeignKeyConstraints(true).Lazy(false).Eager(false).Persistence(true).
		OrphanedRowAction(OrphanedRowActionNullify).Cascade(CascadeRecover)
	if got != o {
		t.Error("chained calls should all return the same *ForeignKeyOptions instance")
	}
}
