package entity

import (
	"strings"

	"github.com/leandroluk/golem"
)

// pendingColumn defers a Col() declaration's final ColumnMeta resolution
// until after the user's callback has finished running. This is required
// because Col returns a *Column that the caller may still chain
// .Name(...) onto after Col itself returns.
type pendingColumn struct {
	fieldName string
	colType   golem.ColumnType
	cb        *Column
}

// pendingIndex defers Index() resolution until finalize().
type pendingIndex struct {
	fieldNames []string
	ib         *Index
}

// Table is passed into the callback given to New. Every method resolves
// its fieldPtr argument(s) against the same zero-value instance New created.
type Table struct {
	zero any // *T, same instance New created

	setTableName    func(string)
	setSchemaName   func(string)
	setPrimaryKey   func([]string)
	setForeignKey   func([]ForeignKeyMeta)
	setColumns      func([]ColumnMeta)
	setUniques      func([][]string)
	setIndexes      func([]IndexMeta)
	setCreateDate   func(string)
	setUpdateDate   func(string)
	setDeleteDate   func(string)

	pendingColumns    []pendingColumn
	pendingPrimaryKey []string // field names, translated to column names at finalize()
	pendingForeignKey []string // field names
	pendingUniques    [][]string // each inner slice is field names
	pendingIndexes    []pendingIndex
	pendingCreateDate string
	pendingUpdateDate string
	pendingDeleteDate string
}

func newTable[T any](zero *T, e *Entity[T]) *Table {
	return &Table{
		zero:          zero,
		setTableName:  func(n string) { e.meta.TableName = n },
		setSchemaName: func(n string) { e.meta.SchemaName = n },
		setPrimaryKey: func(cols []string) { e.meta.PrimaryKey = cols },
		setForeignKey: func(fks []ForeignKeyMeta) { e.meta.ForeignKeys = fks },
		setColumns:    func(cols []ColumnMeta) { e.meta.Columns = cols },
		setUniques:    func(u [][]string) { e.meta.Uniques = u },
		setIndexes:    func(idx []IndexMeta) { e.meta.Indexes = idx },
		setCreateDate: func(f string) { e.meta.CreateDateField = f },
		setUpdateDate: func(f string) { e.meta.UpdateDateField = f },
		setDeleteDate: func(f string) { e.meta.DeleteDateField = f },
	}
}

// TableName overrides the entity's default table name (lowercased struct name).
func (b *Table) TableName(name string) { b.setTableName(name) }

// SchemaName overrides the entity's schema name (unset by default).
func (b *Table) SchemaName(name string) { b.setSchemaName(name) }

// Col maps fieldPtr to a column with an explicit ColumnType. The default
// column name is strings.ToLower(fieldName); override it via the returned
// *Column's .Name(...).
func (b *Table) Col(fieldPtr any, t golem.ColumnType) *Column {
	fieldName, err := ResolveField(b.zero, fieldPtr)
	if err != nil {
		panic(err)
	}

	cb := &Column{}
	b.pendingColumns = append(b.pendingColumns, pendingColumn{
		fieldName: fieldName,
		colType:   t,
		cb:        cb,
	})
	return cb
}

// ForeignKey records that fieldPtr's column references target's primary key.
func (b *Table) ForeignKey(fieldPtr any, target any) {
	fieldName, err := ResolveField(b.zero, fieldPtr)
	if err != nil {
		panic(err)
	}
	b.pendingForeignKey = append(b.pendingForeignKey, fieldName)
}

// PrimaryKey declares the entity's primary key, single or composite.
func (b *Table) PrimaryKey(fieldPtrs ...any) {
	names := make([]string, len(fieldPtrs))
	for i, fp := range fieldPtrs {
		name, err := ResolveField(b.zero, fp)
		if err != nil {
			panic(err)
		}
		names[i] = name
	}
	b.pendingPrimaryKey = names
}

// Unique declares a unique constraint over one or more fields. Multiple calls
// add multiple independent unique constraints.
func (b *Table) Unique(fieldPtrs ...any) {
	names := make([]string, len(fieldPtrs))
	for i, fp := range fieldPtrs {
		name, err := ResolveField(b.zero, fp)
		if err != nil {
			panic(err)
		}
		names[i] = name
	}
	b.pendingUniques = append(b.pendingUniques, names)
}

// Index declares an index over one or more fields. Returns an *Index
// for optional .Name(...) and .Unique() configuration.
func (b *Table) Index(fieldPtrs ...any) *Index {
	names := make([]string, len(fieldPtrs))
	for i, fp := range fieldPtrs {
		name, err := ResolveField(b.zero, fp)
		if err != nil {
			panic(err)
		}
		names[i] = name
	}
	ib := &Index{}
	b.pendingIndexes = append(b.pendingIndexes, pendingIndex{fieldNames: names, ib: ib})
	return ib
}

// CreateDate marks fieldPtr as the column that auto-receives the operation's
// timestamp on INSERT.
func (b *Table) CreateDate(fieldPtr any) {
	fieldName, err := ResolveField(b.zero, fieldPtr)
	if err != nil {
		panic(err)
	}
	b.pendingCreateDate = fieldName
}

// UpdateDate marks fieldPtr as the column that auto-receives the operation's
// timestamp on UPDATE.
func (b *Table) UpdateDate(fieldPtr any) {
	fieldName, err := ResolveField(b.zero, fieldPtr)
	if err != nil {
		panic(err)
	}
	b.pendingUpdateDate = fieldName
}

// DeleteDate marks fieldPtr as the soft-delete timestamp column. Entities
// with a DeleteDate automatically get WHERE deleted_at IS NULL filtering on
// every Where-capable builder (opt-out via .WithDeleted()).
func (b *Table) DeleteDate(fieldPtr any) {
	fieldName, err := ResolveField(b.zero, fieldPtr)
	if err != nil {
		panic(err)
	}
	b.pendingDeleteDate = fieldName
}

// finalize resolves every deferred declaration into the owning Entity's
// EntityMeta. Must run after the user's New callback returns.
func (b *Table) finalize() {
	columns := make([]ColumnMeta, 0, len(b.pendingColumns))
	fieldToColumn := make(map[string]string, len(b.pendingColumns))

	for _, pc := range b.pendingColumns {
		name := pc.cb.ResolvedName()
		if name == "" {
			name = strings.ToLower(pc.fieldName)
		}
		defaultVal, hasDefault := pc.cb.ResolvedDefault()
		columns = append(columns, ColumnMeta{
			FieldName:   pc.fieldName,
			Name:        name,
			Type:        pc.colType,
			Nullable:    pc.cb.ResolvedNullable(),
			Default:     defaultVal,
			HasDefault:  hasDefault,
			DefaultFunc: pc.cb.ResolvedDefaultFunc(),
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

	if len(b.pendingUniques) > 0 {
		uniques := make([][]string, len(b.pendingUniques))
		for i, fieldNames := range b.pendingUniques {
			cols := make([]string, len(fieldNames))
			for j, fn := range fieldNames {
				if colName, ok := fieldToColumn[fn]; ok {
					cols[j] = colName
				} else {
					cols[j] = strings.ToLower(fn)
				}
			}
			uniques[i] = cols
		}
		b.setUniques(uniques)
	}

	if len(b.pendingIndexes) > 0 {
		indexes := make([]IndexMeta, len(b.pendingIndexes))
		for i, pi := range b.pendingIndexes {
			cols := make([]string, len(pi.fieldNames))
			for j, fn := range pi.fieldNames {
				if colName, ok := fieldToColumn[fn]; ok {
					cols[j] = colName
				} else {
					cols[j] = strings.ToLower(fn)
				}
			}
			indexes[i] = IndexMeta{
				Columns: cols,
				Name:    pi.ib.ResolvedName(),
				Unique:  pi.ib.IsUnique(),
			}
		}
		b.setIndexes(indexes)
	}

	if b.pendingCreateDate != "" {
		b.setCreateDate(b.pendingCreateDate)
	}
	if b.pendingUpdateDate != "" {
		b.setUpdateDate(b.pendingUpdateDate)
	}
	if b.pendingDeleteDate != "" {
		b.setDeleteDate(b.pendingDeleteDate)
	}
}

