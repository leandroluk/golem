# Spec: Zero-Allocation Scanner & Lock-Free Cache (M22)

**ID:** M22  
**Scope:** Large  
**Status:** Specify

---

## Contexto

Análise do BreezeORM (clonado em `c:\dev\breezeorm`) identificou técnicas de performance
que reduzem alocações em operações de leitura sem comprometer a type-safety que é o diferencial
do Golem em relação ao GORM/BreezeORM.

O Golem hoje usa `repository.assignFieldValue` para mapear colunas de resultado em campos de struct.
Esse caminho invoca `reflect.NewAt(t, ptr).Interface()` por campo por linha — boxing que aloca em heap
toda vez. O BreezeORM resolve isso pré-classificando cada campo em um `fieldKind` (enum) e usando
`(*int64)(unsafe.Add(base, offset))` direto no scan, eliminando o reflect no path quente.

---

## Problema a Resolver

### REQ-001 — Scanner por-coluna com alocação zero no path quente

Toda leitura de `FindMany`/`FindOne`/`Exec` (Repository) percorre `repository.assignFieldValue`,
que hoje usa reflect para cada campo de cada linha. Para uma query que retorna 50 linhas com 10
colunas, isso é 500 alocações desnecessárias.

**Aceitação:** `FindMany` com 50 linhas deve alocar ≤ `N_linhas × N_colunas_exóticas` vezes no
scanner (onde "exótico" = UUID/JSON/tipos não-primitivos), não `N_linhas × N_colunas_total`.

---

### REQ-002 — Reuso do `[]any` de scan entre linhas (`sync.Pool`)

Cada chamada a `rows.Scan(targets...)` precisa de um `[]any`. Atualmente esse slice é alocado por
linha. Um `sync.Pool` elimina essa alocação no regime de alta concorrência / loop de linhas.

**Aceitação:** O pool deve ser por `Repository[T]` (ou por shape de query), não global.

---

### REQ-003 — Pré-alocação de slice com hint do LIMIT

`FindMany` com `Limit(N)` já sabe o tamanho máximo do resultado. O slice de retorno deve ser
pré-alocado com `make([]T, 0, N)` para evitar os reallocs do `append` padrão.

**Aceitação:** `FindMany` com `Limit(50)` deve chamar `make([]T, 0, 50)` (não `make([]T, 0, 0)`
+ `append`).

---

### REQ-004 — Registry de entidades lock-free no path de leitura

`entity.ForeignKeysReferencing(targetTable)` e quaisquer lookups de metadata de entity hoje usam
`sync.RWMutex`. O path de leitura (hot) deve ser lock-free via `atomic.Pointer[map[...]]`, com
escrita serializada via mutex (copy-on-write). Mesmo padrão de `pkg/metadata/metadata.go` do
BreezeORM e de `pkg/cache/cache.go`.

**Aceitação:** Zero `RLock()` no path de leitura pós-primeiro-registro de entidades.

---

### REQ-005 — Documentar e marcar `Dialect.Bind`/`Scan` como dead code (cleanup)

AD-037 já documentou que `Bind`/`Scan` nunca são chamados em produção. Antes de otimizar o
scanner, precisamos deixar explícito que o novo scanner **não** vai através do `Dialect.Bind`/`Scan`
— evita confusão futura.

**Aceitação:** Comentário `// dead code — ver AD-037 e M22-design.md` nos dois métodos em todos os
adapters, + um test lint que falha se algum adapter chamar `Bind`/`Scan` num path não-test.

---

## Fora de Escopo

- Query optimizer (logical planner + 8 passes) — complexidade arquitetural alta, ganho incremental.
- Prepared statement cache — depende de canonical ordering; deferred para M23 se M22 validar ROI.
- `Dialect.Bind`/`Scan` remoção — breaking change de API pública; deferred, requer major version bump.
- BreezeORM tem `ScanOneFast`/`ScanAllHintFast` com code-gen; code-gen está fora de escopo aqui.

---

## Impacto Esperado

Baseado nos benchmarks do BreezeORM e na análise do código golem:

| Operação | Antes (estimado) | Depois (estimado) | Redução allocs |
|---|:---:|:---:|:---:|
| `FindOne` (5 colunas primitivas) | ~5 allocs/scan | ~0 allocs/scan | ~100% |
| `FindMany(50)` (5 colunas primitivas) | ~250 allocs | ~0 allocs scanner | ~100% |
| `FindMany(50)` (com UUID/JSON) | ~300 allocs | ~200 allocs | ~33% |
| Concorrência (goroutines competindo) | N locks/read | 0 locks/read | 100% |

---

## Restrições

- Manter 100% de cobertura (AD-026) — todos os novos paths precisam de testes diretos.
- Manter multi-dialect: o scanner deve funcionar para todos os 5 adapters.
- `unsafe` só em código interno (`internal/`), nunca exposto na API pública.
- Nenhuma mudança de API pública (backward compatible M22).
- A suíte de conformidade `internal/dialecttest` deve continuar verde para todos os 5 adapters.
