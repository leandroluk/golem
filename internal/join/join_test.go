package join

import (
	"testing"
	"time"

	golem "github.com/leandroluk/golem/internal/core"
	"github.com/leandroluk/golem/internal/entity"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
)

type joinTestParent struct {
	ID int64
}

var joinTestParentEntity = entity.New(func(p *joinTestParent, b *entity.Table) {
	b.TableName("parents")
	b.Col(&p.ID, golem.BIGINT())
	b.PrimaryKey(&p.ID)
})

type joinTestChild struct {
	ID        int64
	ParentID  int64
	DeletedAt *time.Time
}

var joinTestChildEntity = entity.New(func(c *joinTestChild, b *entity.Table) {
	b.TableName("children")
	b.Col(&c.ID, golem.BIGINT())
	b.Col(&c.ParentID, golem.BIGINT())
	b.Col(&c.DeletedAt, golem.DATETIME()).Name("deleted_at")
	b.PrimaryKey(&c.ID)
	b.DeleteDate(&c.DeletedAt)
})

func TestInner_RegistersJoinData(t *testing.T) {
	q := query.New[joinTestParent]()

	Inner(q, joinTestChildEntity, func(c *joinTestChild, q1 *query.Join[joinTestChild]) {
		q1.On(&c.ParentID, &c.ID)
		q1.Where(op.Eq(&c.ID, int64(5)))
	})

	joins := q.Joins()
	if len(joins) != 1 {
		t.Fatalf("expected 1 join registered, got %d", len(joins))
	}
	jd := joins[0]
	if jd.Type != "inner" {
		t.Errorf("Type = %q, want inner", jd.Type)
	}
	if jd.TableName != "children" {
		t.Errorf("TableName = %q, want children", jd.TableName)
	}
	if _, ok := jd.Zero.(*joinTestChild); !ok {
		t.Errorf("Zero = %T, want *joinTestChild", jd.Zero)
	}
	if len(jd.OnFieldPairs) != 1 {
		t.Fatalf("expected 1 On field pair, got %d", len(jd.OnFieldPairs))
	}
	if len(jd.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(jd.Conditions))
	}
	if jd.WithDeleted {
		t.Errorf("WithDeleted = true, want false (not called)")
	}
	if jd.DeleteDateField != "DeletedAt" {
		t.Errorf("DeleteDateField = %q, want DeletedAt", jd.DeleteDateField)
	}
	wantF2C := map[string]string{"ID": "id", "ParentID": "parentid", "DeletedAt": "deleted_at"}
	for field, col := range wantF2C {
		if jd.FieldToColumn[field] != col {
			t.Errorf("FieldToColumn[%q] = %q, want %q", field, jd.FieldToColumn[field], col)
		}
	}
}

func TestInner_WithDeleted_PropagatesFlag(t *testing.T) {
	q := query.New[joinTestParent]()

	Inner(q, joinTestChildEntity, func(c *joinTestChild, q1 *query.Join[joinTestChild]) {
		q1.On(&c.ParentID, &c.ID)
		q1.WithDeleted()
	})

	if !q.Joins()[0].WithDeleted {
		t.Errorf("WithDeleted = false, want true (WithDeleted() was called)")
	}
}

func TestLeft_RegistersJoinDataWithLeftType(t *testing.T) {
	q := query.New[joinTestParent]()

	Left(q, joinTestChildEntity, func(c *joinTestChild, q1 *query.Join[joinTestChild]) {
		q1.On(&c.ParentID, &c.ID)
	})

	joins := q.Joins()
	if len(joins) != 1 {
		t.Fatalf("expected 1 join registered, got %d", len(joins))
	}
	if joins[0].Type != "left" {
		t.Errorf("Type = %q, want left", joins[0].Type)
	}
}

func TestRight_RegistersJoinDataWithRightType(t *testing.T) {
	q := query.New[joinTestParent]()

	Right(q, joinTestChildEntity, func(c *joinTestChild, q1 *query.Join[joinTestChild]) {
		q1.On(&c.ParentID, &c.ID)
	})

	joins := q.Joins()
	if len(joins) != 1 {
		t.Fatalf("expected 1 join registered, got %d", len(joins))
	}
	if joins[0].Type != "right" {
		t.Errorf("Type = %q, want right", joins[0].Type)
	}
}

func TestFull_RegistersJoinDataWithFullType(t *testing.T) {
	q := query.New[joinTestParent]()

	Full(q, joinTestChildEntity, func(c *joinTestChild, q1 *query.Join[joinTestChild]) {
		q1.On(&c.ParentID, &c.ID)
	})

	joins := q.Joins()
	if len(joins) != 1 {
		t.Fatalf("expected 1 join registered, got %d", len(joins))
	}
	if joins[0].Type != "full" {
		t.Errorf("Type = %q, want full", joins[0].Type)
	}
}

func TestInner_NoOnOrConditions_RegistersEmptySlices(t *testing.T) {
	q := query.New[joinTestParent]()

	Inner(q, joinTestChildEntity, func(c *joinTestChild, q1 *query.Join[joinTestChild]) {})

	jd := q.Joins()[0]
	if len(jd.OnFieldPairs) != 0 {
		t.Errorf("expected 0 On field pairs, got %d", len(jd.OnFieldPairs))
	}
	if len(jd.Conditions) != 0 {
		t.Errorf("expected 0 conditions, got %d", len(jd.Conditions))
	}
}
