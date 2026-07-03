package entity

import (
	"sync"

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
	fkRegistry   = map[string][]FKRegistration{}
)

// registerForeignKey is called from Table.finalize() for every declared
// ForeignKey, once the child entity's own table/column names are resolved.
func registerForeignKey(reg FKRegistration) {
	fkRegistryMu.Lock()
	defer fkRegistryMu.Unlock()
	fkRegistry[reg.TargetTableName] = append(fkRegistry[reg.TargetTableName], reg)
}

// ForeignKeysReferencing returns every registered ForeignKey pointing at
// targetTable, in registration order. Used by Repository[T].Delete to apply
// OnDelete/OnUpdate cascade behavior.
func ForeignKeysReferencing(targetTable string) []FKRegistration {
	fkRegistryMu.Lock()
	defer fkRegistryMu.Unlock()
	regs := fkRegistry[targetTable]
	out := make([]FKRegistration, len(regs))
	copy(out, regs)
	return out
}
