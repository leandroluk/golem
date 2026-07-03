package entity

import (
	"strings"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/column"
)

// pendingColumn defers a Col() declaration's final ColumnMeta resolution
// until after the user's callback has finished running. This is required
// because Col returns a *column.Builder that the caller may still chain
// .Name(...) onto after Col itself returns — reading column.Builder's
// ResolvedName() synchronously inside Col would miss any such override.
type pendingColumn struct {
	fieldName string
	colType   golem.ColumnType
	cb        *column.Builder
}

// Builder is passed into the callback given to New. Every method resolves
// its fieldPtr argument(s) against the same zero-value instance New created.
//
// Builder itself is not generic over T: making it so would force New's
// callback signature (and every other consumer of *Builder) to thread T
// through as well, for no benefit — Builder never needs to construct a T,
// only resolve pointers against one and mutate the Entity[T] that owns it.
// That mutation is expressed as a handful of closures captured in
// newBuilder, which is generic (it has access to T via its own type
// parameter at construction time) even though the struct it returns isn't.
type Builder struct {
	zero any // *T, same instance New created — needed for resolveField's offset math

	setTableName  func(string)
	setSchemaName func(string)
	setPrimaryKey func([]string)
	setForeignKey func([]ForeignKeyMeta)
	setColumns    func([]ColumnMeta)

	pendingColumns    []pendingColumn
	pendingPrimaryKey []string // field names, translated to column names at finalize()
	pendingForeignKey []string // field names
}

func newBuilder[T any](zero *T, e *Entity[T]) *Builder {
	return &Builder{
		zero:          zero,
		setTableName:  func(n string) { e.meta.TableName = n },
		setSchemaName: func(n string) { e.meta.SchemaName = n },
		setPrimaryKey: func(cols []string) { e.meta.PrimaryKey = cols },
		setForeignKey: func(fks []ForeignKeyMeta) { e.meta.ForeignKeys = fks },
		setColumns:    func(cols []ColumnMeta) { e.meta.Columns = cols },
	}
}

// TableName overrides the entity's default table name (lowercased struct
// name).
func (b *Builder) TableName(name string) { b.setTableName(name) }

// SchemaName overrides the entity's schema name (unset by default, meaning
// the connection/adapter default applies).
func (b *Builder) SchemaName(name string) { b.setSchemaName(name) }

// Col maps fieldPtr to a column with an explicit ColumnType. The default
// column name is strings.ToLower(fieldName); override it via the returned
// *column.Builder's .Name(...). The final column name is only resolved
// after the entire New callback finishes (see pendingColumn's doc comment),
// so calling .Name(...) on the returned builder at any point before New
// returns is safe.
func (b *Builder) Col(fieldPtr any, t golem.ColumnType) *column.Builder {
	fieldName, err := resolveField(b.zero, fieldPtr)
	if err != nil {
		panic(err) // programmer error — fieldPtr must belong to T
	}

	cb := &column.Builder{}
	b.pendingColumns = append(b.pendingColumns, pendingColumn{
		fieldName: fieldName,
		colType:   t,
		cb:        cb,
	})
	return cb
}

// ForeignKey records that fieldPtr's column references target's primary
// key. target is accepted as `any` (rather than a strongly-typed
// *Entity[J]) because Builder is not generic and a single non-generic
// method cannot introduce its own type parameter in Go. This pass only
// needs to know THAT a field is a foreign key (per schema-declaration
// spec.md AC-3), not walk through to the target's metadata — the
// repository-core-crud feature's Insert/FindByID treat FK columns as plain
// columns. target is therefore accepted but not dereferenced or stored;
// only fieldPtr is resolved (so a bad fieldPtr still panics consistently
// with Col/PrimaryKey).
func (b *Builder) ForeignKey(fieldPtr any, target any) {
	fieldName, err := resolveField(b.zero, fieldPtr)
	if err != nil {
		panic(err)
	}
	b.pendingForeignKey = append(b.pendingForeignKey, fieldName)
}

// PrimaryKey declares the entity's primary key, single or composite, in the
// given order. Field names are resolved now; translation to final COLUMN
// names (which may differ via a Col(...).Name(...) override) happens at
// finalize(), once every Col call's default-vs-override has settled.
func (b *Builder) PrimaryKey(fieldPtrs ...any) {
	names := make([]string, len(fieldPtrs))
	for i, fp := range fieldPtrs {
		name, err := resolveField(b.zero, fp)
		if err != nil {
			panic(err)
		}
		names[i] = name
	}
	b.pendingPrimaryKey = names
}

// finalize resolves every deferred declaration (Col, PrimaryKey,
// ForeignKey) into the owning Entity's EntityMeta. It must run after the
// user's New callback returns, so that any column.Builder.Name(...) calls
// chained onto a Col(...) result have already taken effect.
func (b *Builder) finalize() {
	columns := make([]ColumnMeta, 0, len(b.pendingColumns))
	fieldToColumn := make(map[string]string, len(b.pendingColumns))

	for _, pc := range b.pendingColumns {
		name := pc.cb.ResolvedName()
		if name == "" {
			name = strings.ToLower(pc.fieldName)
		}
		columns = append(columns, ColumnMeta{
			FieldName: pc.fieldName,
			Name:      name,
			Type:      pc.colType,
		})
		fieldToColumn[pc.fieldName] = name
	}
	b.setColumns(columns)

	if b.pendingPrimaryKey != nil {
		pkCols := make([]string, len(b.pendingPrimaryKey))
		for i, fieldName := range b.pendingPrimaryKey {
			if colName, ok := fieldToColumn[fieldName]; ok {
				pkCols[i] = colName
			} else {
				pkCols[i] = strings.ToLower(fieldName)
			}
		}
		b.setPrimaryKey(pkCols)
	}

	if len(b.pendingForeignKey) > 0 {
		fks := make([]ForeignKeyMeta, len(b.pendingForeignKey))
		for i, fieldName := range b.pendingForeignKey {
			fks[i] = ForeignKeyMeta{FieldName: fieldName}
		}
		b.setForeignKey(fks)
	}
}
