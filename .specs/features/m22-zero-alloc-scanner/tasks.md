# Tasks: M22 — Zero-Allocation Scanner & Lock-Free Cache

**Última atualização:** 2026-07-15  
**Status global:** Planejado

---

## Sumário de Dependências

```
T1 (entity.ColumnMeta: GoType + Offset)
  └─ T2 (internal/scanner: Plan, fieldKind, ScanFromMap)
       └─ T3 (repository: scanRow → scanner; FindMany ScanAllHint)
            └─ T4 (entity fkRegistry: lock-free atomic.Pointer)
                 └─ T5 (dead code comments Bind/Scan + lint test)
                      └─ T6 (bench + conformance full green)
```

---

## Tarefas

### T1 — Adicionar `GoType` e `Offset` ao `entity.ColumnMeta`

**O quê:** `ColumnMeta` ganha dois novos campos internos. O `finalize()` em `table.go` resolve o
offset e o `reflect.Type` do campo usando a mesma aritmética de `ResolveField`.

**Onde:**
- [MODIFY] `entity/entity.go` — adicionar `GoType reflect.Type` e `Offset uintptr` em `ColumnMeta`
- [MODIFY] `entity/table.go` — em `finalize()`, popular os dois novos campos:
  ```go
  t := reflect.TypeOf(b.zero).Elem()
  for i := 0; i < t.NumField(); i++ {
      if t.Field(i).Name == pc.fieldName {
          // GoType = t.Field(i).Type
          // Offset = t.Field(i).Offset
      }
  }
  ```
- [MODIFY] `entity/entity_test.go` — verificar que `ColumnMeta.Offset != 0` para campos não-primeiros

**Reutiliza:** `ResolveField`'s mesma aritmética — zero código duplicado, só expõe o que já era calculado.

**Done when:**
- `entity.New[T]()` produz `ColumnMeta.Offset` correto para todos os campos da struct
- `ColumnMeta.GoType` é não-nil para todas as colunas declaradas
- `go test ./entity/...` passa com 100% de cobertura

**Gate:** `go test ./entity/... -cover` ≥ 100%

---

### T2 — `internal/scanner`: `fieldKind`, `Plan`, `ScanFromMap`, `assignFast`

**O quê:** Novo pacote `internal/scanner`. Implementa o path de scan zero-alloc baseado em
`unsafe.Add(base, offset)` + switch de `fieldKind`. Fallback para reflect nos tipos exóticos.

**Onde:**
- [NEW] `internal/scanner/scanner.go`
- [NEW] `internal/scanner/scanner_test.go` (100% coverage direto, não via repository)

**Detalhes de implementação:**

`fieldKind` cobre: `kindInt`, `kindInt8`, `kindInt16`, `kindInt32`, `kindInt64`,
`kindUint`, `kindUint8`, `kindUint16`, `kindUint32`, `kindUint64`,
`kindFloat32`, `kindFloat64`, `kindString`, `kindBool`, `kindBytes` (`[]byte`), `kindTime` (`time.Time`),
`kindNullString`, `kindNullInt64`, `kindNullFloat64`, `kindNullBool`, `kindNullTime`,
`kindOther` (fallback reflect).

`classifyGoType(t reflect.Type) fieldKind` — mapeamento direto.

`Compile(meta entity.EntityMeta) *Plan` — pre-classifica todos os campos.

`ScanFromMap[T any](p *Plan, row map[string]any) (T, error)` — hot loop com `assignFast`.

`assignFast` — switch inlineado. Para `kindOther`, chama a mesma lógica de `assignFieldValue`
(copiada/importada de `repository`, não duplicada — extrair para `internal/scanner` se necessário).

**Done when:**
- `scanner.ScanFromMap` produz o mesmo resultado que `repository.scanRow` para todos os tipos
- `BenchmarkScanFromMap_AllPrimitives` mostra ≤ 0 allocs/op para structs só primitivas
- `go test ./internal/scanner/... -cover` = 100%

**Gate:** `go test ./internal/scanner/... -cover -benchmem -bench=.`

---

### T3 — `repository`: integra scanner + `FindMany` pré-aloca com hint

**O quê:**
1. `Repository[T]` ganha campo `scanPlan *scanner.Plan`, inicializado em `Get[T]`
2. `scanRow` substituído por `scanner.ScanFromMap[T](r.scanPlan, row)`
3. `aggregate.go` usa `scanner.ScanFromMap` também (para o `R` tipo resultado)
4. `FindMany`: pré-aloca slice com `make([]T, 0, cap)` onde `cap` vem do `sel.Limit`

**Onde:**
- [MODIFY] `repository/repository.go`:
  - `Repository[T]` struct: +`scanPlan *scanner.Plan`
  - `Get[T]`: +`scanPlan: scanner.Compile(e.Describe())`
  - `scanRow(row)`: delega para `scanner.ScanFromMap[T]`
  - `FindMany`: pré-aloca resultado com hint
- [MODIFY] `repository/aggregate.go`: mesma substituição para scan de `R`

**Done when:**
- `go test ./repository/... -cover` = 100%
- `task test-integration` (todos os 5 adapters) ainda verde

**Gate:** `task test-integration`

---

### T4 — `entity` fkRegistry: `sync.RWMutex` → `atomic.Pointer[map]`

**O quê:** Migrar o `dataSourceRegistry` e o `fkRegistry` (ambos protegidos por `sync.RWMutex`)
para o padrão lock-free de leitura com `atomic.Pointer[map[K]V]` + `sync.Mutex` apenas para escrita
(copy-on-write).

**Onde:**
- [MODIFY] `entity/entity.go` (ou onde `fkRegistry` vive) — migrar para `atomic.Pointer`
- [MODIFY] `data_source.go` — migrar `dataSourceRegistry` também

**Done when:**
- `ForeignKeysReferencing(targetTable)` não chama `RLock` (verificável via race detector `go test -race`)
- `GetDataSource` não chama `RLock`
- `go test -race ./...` passa sem data race reports

**Gate:** `go test -race ./... -short`

---

### T5 — Dead code comment `Bind`/`Scan` em todos os adapters

**O quê:** Adicionar `// dead code — never called in production path (see AD-037, M22/design.md)`
nos métodos `Bind` e `Scan` de todos os 5 adapters.

**Onde:**
- [MODIFY] `driver/postgres/dialect.go`
- [MODIFY] `driver/mysql/dialect.go`
- [MODIFY] `driver/sqlite/dialect.go`
- [MODIFY] `driver/mssql/dialect.go`
- [MODIFY] `driver/oracle/dialect.go`

**Done when:** Comentário presente em todos os 5 adapters. Nenhum teste quebra.

**Gate:** `go build ./...`

---

### T6 — Benchmarks + conformance full + STATE.md update

**O quê:**
1. Criar `internal/scanner/bench_test.go` com benchmarks comparativos
2. Rodar `task test-integration` completo (todos os 5 adapters)
3. Atualizar `STATE.md` com AD-053 (decisões de M22)
4. Atualizar `ROADMAP.md` com M22 DONE

**Onde:**
- [NEW] `internal/scanner/bench_test.go`
- [MODIFY] `.specs/project/STATE.md`
- [MODIFY] `.specs/project/ROADMAP.md`

**Done when:**
- `task test-integration` verde para todos os 5 adapters
- Benchmark mostra ≤ 0 allocs/op para `ScanFromMap` em structs primitivas
- STATE.md atualizado

**Gate:** `task test-integration` + `go test -bench=. -benchmem ./internal/scanner/...`

---

## Checklist de Execução

- `[x]` T1 — entity.ColumnMeta: GoType + Offset
- `[x]` T2 — internal/scanner (Plan, fieldKind, ScanFromMap)
- `[x]` T3 — repository integra scanner + FindMany hint
- `[x]` T4 — entity/datasource registry lock-free
- `[x]` T5 — dead code comments Bind/Scan
- `[x]` T6 — benchmarks + conformance + STATE.md
