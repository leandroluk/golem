package entity

import (
	"sync"
	"sync/atomic"

	"github.com/leandroluk/golem/relation"
)

// FKRegistration records one child-to-parent ForeignKey relationship,
// captured as a side effect of entity.New running Table.ForeignKey. It's
// indexed by TargetTableName (the parent side) so parent-side operations
// (Repository[T].Delete's cascade logic) can discover which child tables
// reference a given entity — the declaration direction (child declares the
// FK pointing at its parent) is the opposite of the direction cascade needs
// to look things up in, hence the registry.
type FKRegistration struct {
	ChildTableName        string
	ChildColumn           string
	ChildDeleteDateColumn string // "" if the child entity has no soft-delete field
	TargetTableName       string
	Options               *relation.ForeignKeyOptions
}

var (
	fkRegistryMu sync.Mutex
	fkRegistry   atomic.Pointer[map[string][]FKRegistration]
)

func init() {
	m := make(map[string][]FKRegistration)
	fkRegistry.Store(&m)
}

// registerForeignKey is called from Table.finalize() for every declared
// ForeignKey, once the child entity's own table/column names are resolved.
func registerForeignKey(reg FKRegistration) {
	fkRegistryMu.Lock()
	defer fkRegistryMu.Unlock()

	oldMap := *fkRegistry.Load()
	newMap := make(map[string][]FKRegistration, len(oldMap))
	for k, v := range oldMap {
		newMap[k] = v
	}

	oldList := newMap[reg.TargetTableName]
	newList := make([]FKRegistration, len(oldList), len(oldList)+1)
	copy(newList, oldList)
	newList = append(newList, reg)
	newMap[reg.TargetTableName] = newList

	fkRegistry.Store(&newMap)
}

// ForeignKeysReferencing returns every registered ForeignKey pointing at
// targetTable, in registration order. Used by Repository[T].Delete to apply
// OnDelete/OnUpdate cascade behavior.
func ForeignKeysReferencing(targetTable string) []FKRegistration {
	m := *fkRegistry.Load()
	regs := m[targetTable]
	out := make([]FKRegistration, len(regs))
	copy(out, regs)
	return out
}
