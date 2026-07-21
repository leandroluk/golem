package dialecttest

import (
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/entity"
	"github.com/leandroluk/golem/internal/relation"
)

// Widget covers every golem.ColumnType kind in one entity, for
// BindScanRoundTrip, and doubles as the CRUD/Aggregates group's subject
// (Category/Score give Aggregates a groupable column and a summable one).
type Widget struct {
	ID       int64
	Name     string // varchar
	Bio      string // text
	Category string // varchar, groupable
	Score    int64  // bigint, summable
	Price    float64
	Ratio    float64
	Active   bool
	Code     string // char
	Born     time.Time
	Seen     time.Time
	Duration time.Time
	Data     []byte
	UID      string
	Meta     string // json
}

var widgetEntity = entity.New(func(t *Widget, b *entity.Table) {
	b.TableName("conf_widget")
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.Col(&t.Bio, golem.TEXT())
	b.Col(&t.Category, golem.VARCHAR(50))
	b.Col(&t.Score, golem.BIGINT())
	b.Col(&t.Price, golem.DECIMAL(10, 2))
	b.Col(&t.Ratio, golem.FLOAT(10))
	b.Col(&t.Active, golem.BOOLEAN())
	b.Col(&t.Code, golem.CHAR(8))
	b.Col(&t.Born, golem.DATE())
	b.Col(&t.Seen, golem.DATETIME())
	b.Col(&t.Duration, golem.TIME())
	b.Col(&t.Data, golem.BLOB())
	b.Col(&t.UID, golem.UUID())
	b.Col(&t.Meta, golem.JSON())
	b.PrimaryKey(&t.ID)
})

// Deleted is the SoftDelete group's subject.
type Deleted struct {
	ID        int64
	Name      string
	DeletedAt *time.Time
}

var deletedEntity = entity.New(func(t *Deleted, b *entity.Table) {
	b.TableName("conf_deleted")
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.Col(&t.DeletedAt, golem.DATETIME()).Name("deleted_at").Nullable().Default(nil)
	b.PrimaryKey(&t.ID)
	b.DeleteDate(&t.DeletedAt)
})

// Parent is the Cascade/Joins/Preload groups' parent side.
type Parent struct {
	ID   int64
	Name string
}

var parentEntity = entity.New(func(t *Parent, b *entity.Table) {
	b.TableName("conf_parent")
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.PrimaryKey(&t.ID)
	// Unique(&t.Name) exists purely so ConflictDetection has something to
	// violate — no other group depends on Name actually being unique.
	b.Unique(&t.Name)
})

// Child is the Cascade/Joins/Preload groups' child side. 3 separate
// entity.Entity[Child] values exist below (cascadeChildEntity/
// setNullChildEntity/restrictChildEntity), one per OnDelete mode, each
// pointing at its own physical table — the FK's OnDelete behavior is fixed
// per entity mapping (via the FK registry entity.New populates), not
// per-row, so testing all 3 modes needs 3 separate declarations. Matches
// repository/repository_test.go's cascade_child_delete/_setnull/_restrict
// pattern.
type Child struct {
	ID int64
	// ParentID is *int64 (not int64) so SetNull's real result — the column
	// going NULL after the parent is deleted — round-trips through
	// FindOne as a nil pointer, instead of needing raw dialect-specific SQL
	// to observe it.
	ParentID *int64
	Name     string
}

func newChildEntity(tableName string, opts *relation.ForeignKeyOptions) *entity.Entity[Child] {
	return entity.New(func(t *Child, b *entity.Table) {
		b.TableName(tableName)
		b.Col(&t.ID, golem.BIGINT())
		b.Col(&t.ParentID, golem.BIGINT()).Name("parent_id")
		b.Col(&t.Name, golem.VARCHAR(50))
		b.PrimaryKey(&t.ID)
		b.ForeignKey(&t.ParentID, parentEntity, opts)
	})
}

var (
	cascadeChildEntity  = newChildEntity("conf_cascade_child", relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))
	setNullChildEntity  = newChildEntity("conf_setnull_child", relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteSetNull))
	restrictChildEntity = newChildEntity("conf_restrict_child", relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))
)
