# Design: Zero-Allocation Scanner & Lock-Free Cache (M22)

## Diagnóstico: O Path Atual

```
FindMany/FindOne
  → dialect.CompileSelect → sql, args
  → dialect.ExecRaw / QueryContext → []map[string]any (rows)
  → repository.scanRow(row map[string]any) T
      → para cada col: reflect.ValueOf(&result).Elem().FieldByName(col.FieldName)
          → assignFieldValue(field reflect.Value, raw any, ...) error
              → reflect.ValueOf(raw)          ← aloca no heap (boxing)
              → reflect.Value.Set(rawVal)     ← pode alocar
```

**Raiz do problema:** `dialect.ExecRaw` (em todos os adapters) retorna `[]map[string]any` — os valores já vêm boxeados numa map. O repository então chama `reflect.FieldByName` + `reflect.ValueOf(raw)` por campo por linha.

O BreezeORM resolve pulando a map e indo direto do `*sql.Rows` para ponteiros unsafe de campos. Para o Golem, que ainda passa por `[]map[string]any`, há dois níveis de melhoria possíveis:

- **Nível 1 (fácil, menor impacto):** Otimizar `scanRow` para usar offsets e unsafe quando o tipo bate exato — ainda parte de `[]map[string]any`, mas box menos.
- **Nível 2 (maior impacto):** Novo path de scan direto `*sql.Rows → struct`, sem a conversão para `map[string]any` no meio. Isso requer uma nova assinatura em `Dialect` ou um wrapper de scan interno.

### Decisão Arquitetural

**M22 adota o Nível 1 para `scanRow` + o Nível 2 para `FindMany`/`FindOne` de forma opt-in.**

Razão: A assinatura `Dialect.Insert`/`Update` retornando `map[string]any` é necessária para os adapters sem RETURNING (MySQL/Oracle), onde o row retornado vem de um `SELECT` separado. Mas `FindMany`/`FindOne` sempre vêm de um `CompileSelect` + `QueryContext` — essa pipeline pode ter um path direto.

---

## Componentes

### C1 — `internal/scanner` (novo pacote)

Espelha o `pkg/scanner` do BreezeORM, mas adaptado para o modelo de metadados do Golem.

```go
// internal/scanner/scanner.go

type fieldKind uint8

const (
    kindOther fieldKind = iota
    kindInt
    kindInt8
    kindInt16
    kindInt32
    kindInt64
    kindUint
    kindUint8
    kindUint16
    kindUint32
    kindUint64
    kindFloat32
    kindFloat64
    kindString
    kindBool
    kindBytes
    kindTime
    // nullable wrappers
    kindNullString
    kindNullInt64
    kindNullFloat64
    kindNullBool
    kindNullTime
)
```

**`Plan`** — compilado uma vez por `(entity.EntityMeta, resultColumns []string)`:

```go
type FieldAssignment struct {
    ColName   string         // para lookup em map[string]any (path legado)
    Offset    uintptr        // para unsafe path direto
    Kind      fieldKind      // classificado uma vez
    assign    func(unsafe.Pointer) any  // closure pré-resolvida
}

type Plan struct {
    Assignments []FieldAssignment
    targetsPool sync.Pool // reusa []any entre rows
}
```

**`ScanFromMap`** — substitui `scanRow` no repository:

```go
func ScanFromMap[T any](p *Plan, row map[string]any) (T, error) {
    var result T
    base := unsafe.Pointer(&result)
    for i := range p.Assignments {
        a := &p.Assignments[i]
        raw, ok := row[a.ColName]
        if !ok {
            continue
        }
        fp := unsafe.Add(base, a.Offset)
        if err := assignFast(fp, a.Kind, raw); err != nil {
            return result, err
        }
    }
    return result, nil
}
```

**`assignFast`** — switch inlineado com casts diretos:

```go
func assignFast(fp unsafe.Pointer, kind fieldKind, raw any) error {
    switch kind {
    case kindInt64:
        if v, ok := raw.(int64); ok {
            *(*int64)(fp) = v
            return nil
        }
    case kindString:
        if v, ok := raw.(string); ok {
            *(*string)(fp) = v
            return nil
        }
    // ... todos os primitivos ...
    default:
        // fallback reflect — exóticos (UUID, JSON, *T nullable)
        return assignFieldValueUnsafe(fp, kind, raw)
    }
    // type mismatch — tenta conversão reflect
    return assignFieldValueFallback(fp, raw)
}
```

**Nota:** Para campos `*T` (nullable), o path ainda usa reflect — é o `kindOther`. Isso é idêntico ao design do BreezeORM.

---

### C2 — `internal/scanner.CompilePlan` (integra com `entity.EntityMeta`)

```go
// Compile constrói um Plan uma vez por (EntityMeta, resultColumns).
// Deveria ser cacheado pelo caller.
func Compile(meta entity.EntityMeta, resultColumns []string) *Plan {
    colByName := make(map[string]entity.ColumnMeta, len(meta.Columns))
    for _, col := range meta.Columns {
        colByName[col.Name] = col
    }
    
    assignments := make([]FieldAssignment, 0, len(resultColumns))
    for _, name := range resultColumns {
        col, ok := colByName[name]
        if !ok {
            continue
        }
        kind := classifyGoType(col.GoType) // reflect.Type da coluna
        assignments = append(assignments, FieldAssignment{
            ColName: name,
            Offset:  col.Offset, // precomputed em entity.ColumnMeta
            Kind:    kind,
            assign:  assignerFor(kind, col.GoType),
        })
    }
    
    n := len(assignments)
    p := &Plan{
        Assignments: assignments,
    }
    p.targetsPool.New = func() any {
        s := make([]any, n)
        return &s
    }
    return p
}
```

**Requer:** `entity.ColumnMeta` (a struct que representa cada coluna) precisa expor `GoType reflect.Type` e `Offset uintptr`. Verificar se já existe.

---

### C3 — `entity.ColumnMeta` — adicionar `GoType` e `Offset`

Atualmente `entity.EntityMeta` tem `Columns []ColumnMeta`. Verificar se `ColumnMeta` já inclui
`reflect.Type` e o offset unsafe. Se não, adicionar como campos internos acessíveis ao `internal/scanner`.

```go
// entity/entity.go — existente, pode precisar de extensão
type ColumnMeta struct {
    Name      string
    FieldName string
    GoType    reflect.Type  // ← adicionar se não existir
    Offset    uintptr       // ← adicionar se não existir
    // ... outros campos existentes
}
```

---

### C4 — `repository.scanRow` → usa `scanner.ScanFromMap`

A função `scanRow` no `repository.go` é substituída por uma chamada ao `internal/scanner`:

```go
// antes:
func (r *Repository[T]) scanRow(row map[string]any) (T, error) {
    var result T
    v := reflect.ValueOf(&result).Elem()
    for _, col := range r.meta.Columns {
        raw, ok := row[col.Name]
        if !ok { continue }
        if err := assignFieldValue(v.FieldByName(col.FieldName), raw, col.Name, col.FieldName); err != nil {
            return result, err
        }
    }
    return result, nil
}

// depois:
func (r *Repository[T]) scanRow(row map[string]any) (T, error) {
    return scanner.ScanFromMap[T](r.scanPlan, row)
}
```

`r.scanPlan` é compilado uma vez em `Get[T]` e reutilizado:

```go
type Repository[T any] struct {
    conn     golem.Conn
    meta     entity.EntityMeta
    entity   *entity.Entity[T]
    scanPlan *scanner.Plan // ← novo, compilado em Get()
}
```

---

### C5 — `ScanAllHint`: `FindMany` com LIMIT pré-aloca

```go
// repository/repository.go — FindMany
func (r *Repository[T]) FindMany(ctx context.Context, criteria ...func(*T, *query.Query[T])) ([]T, error) {
    // ... build select ...
    limit := sel.Limit // já existe no stmt.Select
    rows, err := r.conn.Dialect().ExecRaw(ctx, r.conn, sql, args)
    // ...
    
    // antes: make([]T, 0)
    // depois:
    cap := 16
    if limit != nil && *limit > 0 {
        cap = *limit
    }
    results := make([]T, 0, cap) // ← pré-aloca com hint
    for _, row := range rawRows {
        t, err := r.scanRow(row)
        // ...
        results = append(results, t)
    }
}
```

---

### C6 — Lock-free FK registry (`entity` package)

O `entity.fkRegistry` (mapa de `targetTable → []FKRegistration`) hoje usa `sync.RWMutex`. 
Migrar para `atomic.Pointer[map[string][]FKRegistration]` + `sync.Mutex` (write only):

```go
// entity/entity.go

var fkRegistry struct {
    snapshot atomic.Pointer[map[string][]FKRegistration]
    writeMu  sync.Mutex
}

func init() {
    empty := map[string][]FKRegistration{}
    fkRegistry.snapshot.Store(&empty)
}

// ForeignKeysReferencing — hot path: zero mutex
func ForeignKeysReferencing(targetTable string) []FKRegistration {
    m := *fkRegistry.snapshot.Load()
    return m[targetTable]
}

// registerFK — cold path: copy-on-write
func registerFK(reg FKRegistration) {
    fkRegistry.writeMu.Lock()
    defer fkRegistry.writeMu.Unlock()
    old := *fkRegistry.snapshot.Load()
    next := make(map[string][]FKRegistration, len(old)+1)
    for k, v := range old {
        next[k] = v
    }
    next[reg.TargetTable] = append(next[reg.TargetTable], reg)
    fkRegistry.snapshot.Store(&next)
}
```

---

### C7 — Marcar `Dialect.Bind`/`Scan` como dead code (REQ-005)

Adicionar comentários em todos os adapters + lint test que verifica que nenhum path de produção
(fora de `_test.go`) chama `Bind`/`Scan` no adapters.

---

## Arquivos Afetados

| Arquivo | Mudança |
|---|---|
| `internal/scanner/scanner.go` | **[NEW]** — fieldKind, Plan, ScanFromMap, assignFast |
| `internal/scanner/scanner_test.go` | **[NEW]** — 100% coverage |
| `entity/entity.go` | Adicionar `GoType`/`Offset` em `ColumnMeta` |
| `repository/repository.go` | `scanRow` → `scanner.ScanFromMap`; `FindMany` pré-aloca com hint; `Repository[T]` ganha `scanPlan` |
| `repository/aggregate.go` | `assignFieldValue` → `scanner.ScanFromMap` |
| `entity/entity.go` (fkRegistry) | Migra de `RWMutex` para `atomic.Pointer` |
| `driver/*/dialect.go` (x5) | Comentário dead code em `Bind`/`Scan` |

---

## Riscos e Mitigações

| Risco | Mitigação |
|---|---|
| `unsafe.Add` + offset errado corrompem memória | `Compile` valida offset via `reflect.Type.Size()` antes de usar |
| `kindOther` (fallback reflect) para tipos exóticos — comportamento idêntico ao atual | Garantido: `kindOther` usa exatamente o mesmo `reflect.NewAt(t,p).Interface()` path que `assignFieldValue` usa hoje |
| Suíte de conformidade quebra num dos 5 adapters | Gate: `task test-integration` deve passar antes de merge |
| `entity.ColumnMeta.Offset` incorreto para campos de struct embedded | Usar `reflect.Type.Field(i).Offset` que já é o offset relativo ao struct raiz (mesmo mecanismo do BreezeORM) |

---

## Verificação de Performance

Após implementação, criar `internal/scanner/bench_test.go` com:

```go
func BenchmarkScanFromMap_Primitives(b *testing.B) { ... }
func BenchmarkScanFromMap_WithUUID(b *testing.B) { ... }
```

Comparar com `BenchmarkAssignFieldValue_Current` (benchmark do path antigo, também novo).

Meta: ≥50% redução de allocs em structs com apenas campos primitivos.
