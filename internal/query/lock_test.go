package query

import "testing"

type lockTestSubject struct {
	ID int64
}

func TestQuery_NoLock_Defaults(t *testing.T) {
	q := New[lockTestSubject]()
	if q.GetLockStrength() != "" {
		t.Errorf("GetLockStrength() = %q, want empty", q.GetLockStrength())
	}
	if q.GetLockWait() != LockWaitBlock {
		t.Errorf("GetLockWait() = %q, want LockWaitBlock", q.GetLockWait())
	}
}

func TestQuery_ForUpdate_SetsStrength(t *testing.T) {
	q := New[lockTestSubject]()
	q.ForUpdate()
	if q.GetLockStrength() != LockForUpdate {
		t.Errorf("GetLockStrength() = %q, want %q", q.GetLockStrength(), LockForUpdate)
	}
	if q.GetLockWait() != LockWaitBlock {
		t.Errorf("GetLockWait() = %q, want LockWaitBlock (default)", q.GetLockWait())
	}
}

func TestQuery_ForUpdate_WithWaitPolicy(t *testing.T) {
	q := New[lockTestSubject]()
	q.ForUpdate(LockWaitNoWait)
	if q.GetLockWait() != LockWaitNoWait {
		t.Errorf("GetLockWait() = %q, want %q", q.GetLockWait(), LockWaitNoWait)
	}
}

func TestQuery_ForNoKeyUpdate(t *testing.T) {
	q := New[lockTestSubject]()
	q.ForNoKeyUpdate()
	if q.GetLockStrength() != LockForNoKeyUpdate {
		t.Errorf("GetLockStrength() = %q, want %q", q.GetLockStrength(), LockForNoKeyUpdate)
	}
}

func TestQuery_ForShare(t *testing.T) {
	q := New[lockTestSubject]()
	q.ForShare(LockWaitSkipLocked)
	if q.GetLockStrength() != LockForShare {
		t.Errorf("GetLockStrength() = %q, want %q", q.GetLockStrength(), LockForShare)
	}
	if q.GetLockWait() != LockWaitSkipLocked {
		t.Errorf("GetLockWait() = %q, want %q", q.GetLockWait(), LockWaitSkipLocked)
	}
}

func TestQuery_ForKeyShare(t *testing.T) {
	q := New[lockTestSubject]()
	q.ForKeyShare()
	if q.GetLockStrength() != LockForKeyShare {
		t.Errorf("GetLockStrength() = %q, want %q", q.GetLockStrength(), LockForKeyShare)
	}
}

func TestQuery_Lock_ChainReturnsSameInstance(t *testing.T) {
	q := New[lockTestSubject]()
	if q.ForUpdate() != q {
		t.Error("ForUpdate should return the same *Query instance")
	}
}

func TestQuery_Lock_LaterCallOverridesEarlier(t *testing.T) {
	q := New[lockTestSubject]()
	q.ForShare()
	q.ForUpdate()
	if q.GetLockStrength() != LockForUpdate {
		t.Errorf("GetLockStrength() = %q, want %q (last call wins)", q.GetLockStrength(), LockForUpdate)
	}
}
