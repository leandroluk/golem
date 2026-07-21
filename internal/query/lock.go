package query

// LockStrength names a row-locking mode for SELECT ... FOR {UPDATE|SHARE|...}.
// The empty value means "no locking requested."
type LockStrength string

const (
	LockForUpdate      LockStrength = "update"
	LockForNoKeyUpdate LockStrength = "no_key_update"
	LockForShare       LockStrength = "share"
	LockForKeyShare    LockStrength = "key_share"
)

// LockWait names how a locking read behaves when it hits an already-locked
// row. The empty value means "block until the lock is released" (Postgres's
// default).
type LockWait string

const (
	LockWaitBlock      LockWait = ""
	LockWaitNoWait     LockWait = "nowait"
	LockWaitSkipLocked LockWait = "skip_locked"
)

// ForUpdate requests SELECT ... FOR UPDATE — an exclusive row lock, held
// until the enclosing transaction ends. wait optionally overrides the
// default blocking behavior (NOWAIT/SKIP LOCKED); only the first value is
// used, extras are ignored (variadic purely to make the argument optional).
//
// Locking a read that isn't running inside a transaction is a no-op that
// looks like it worked (the lock releases the instant the implicit
// single-statement transaction ends) — Repository[T].FindMany/FindOne
// refuse to run a locked query outside a real golem.Tx, returning an error
// instead of silently doing nothing useful.
func (q *Query[T]) ForUpdate(wait ...LockWait) *Query[T] {
	return q.lock(LockForUpdate, wait...)
}

// ForNoKeyUpdate requests SELECT ... FOR NO KEY UPDATE — like ForUpdate,
// but doesn't block concurrent FOR KEY SHARE readers (weaker exclusion).
func (q *Query[T]) ForNoKeyUpdate(wait ...LockWait) *Query[T] {
	return q.lock(LockForNoKeyUpdate, wait...)
}

// ForShare requests SELECT ... FOR SHARE — a shared row lock (blocks
// concurrent writers, allows concurrent shared readers).
func (q *Query[T]) ForShare(wait ...LockWait) *Query[T] {
	return q.lock(LockForShare, wait...)
}

// ForKeyShare requests SELECT ... FOR KEY SHARE — the weakest lock mode,
// blocks only changes to the row's key columns.
func (q *Query[T]) ForKeyShare(wait ...LockWait) *Query[T] {
	return q.lock(LockForKeyShare, wait...)
}

func (q *Query[T]) lock(strength LockStrength, wait ...LockWait) *Query[T] {
	q.lockStrength = strength
	if len(wait) > 0 {
		q.lockWait = wait[0]
	}
	return q
}

// GetLockStrength returns the requested lock strength, or "" if none.
func (q *Query[T]) GetLockStrength() LockStrength { return q.lockStrength }

// GetLockWait returns the requested wait policy (LockWaitBlock if unset).
func (q *Query[T]) GetLockWait() LockWait { return q.lockWait }
