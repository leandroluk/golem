# Design: Flat Root API (M24)

**Spec**: `.specs/features/flat-root-api/spec.md`
**Status**: Draft

---

## Diagnóstico: Layout Atual

```
github.com/leandroluk/golem                 (root -- Conn, Dialect, DataSource, Option,
                                               ColumnType, Result, Tx, Parser (M23), Logger, erros)
github.com/leandroluk/golem/entity           (público -- importa o root: golem.ColumnType, golem.Conn)
github.com/leandroluk/golem/repository       (público -- importa o root: golem.Conn, golem.Dialect)
github.com/leandroluk/golem/relation         (público -- sem dependência do root)
github.com/leandroluk/golem/op               (público -- sem dependência do root)
github.com/leandroluk/golem/query            (público -- sem dependência do root)
github.com/leandroluk/golem/join             (público -- importa entity, query)
github.com/leandroluk/golem/driver/postgres  (público -- importa o root: golem.Dialect, golem.Option)
github.com/leandroluk/golem/driver/{mysql,mssql,sqlite,oracle}  (idem)
```

`entity` e `repository` importam o root HOJE. Se só eles virassem `internal/`, e o root
continuasse sendo o dono de `Conn`/`Dialect`/etc., reexportar os tipos de `internal/entity` no
`golem.go` criaria ciclo (`internal/entity` → `golem` → `internal/entity`). `driver/*` também
importa o root, mas isso NUNCA foi um ciclo — `driver/*` nunca é importado de volta pelo root, é
uma folha (o consumidor final importa `driver/postgres` diretamente, `golem.go` nunca precisa
saber que `driver/postgres` existe).

### Decisão Arquitetural

Separar em DUAS camadas dentro de `internal/`:

1. **`internal/core`** — tudo que hoje mora direto no pacote root (`column_type.go`, `conn.go`,
   `connector.go`, `data_source.go`, `dialect.go`, `errors.go`, `logger.go`, `options.go`,
   `parser.go` do M23). Vira a base — não importa NADA dos outros pacotes internos, só stdlib.
2. **`internal/entity`, `internal/repository`, `internal/relation`, `internal/op`,
   `internal/query`, `internal/join`** — o que hoje são os 6 pacotes públicos. Importam
   `internal/core` no lugar do root sempre que hoje importam `"github.com/leandroluk/golem"`.

`golem.go` (root) importa `internal/core` MAIS os 6 pacotes acima, reexportando tudo — vira 100%
casca, igual `gonest.go` já é hoje no projeto irmão. Regra de dependência (única direção permitida):

```
golem.go (root)  →  internal/core, internal/entity, internal/repository, internal/relation,
                     internal/op, internal/query, internal/join
internal/entity, internal/repository  →  internal/core (nunca o contrário, nunca entre si em ciclo)
internal/repository  →  internal/entity, internal/query   (já é assim hoje, continua)
internal/join  →  internal/entity, internal/query          (já é assim hoje, continua)
driver/*  →  golem (root) — inalterado, continua funcionando via alias
```

---

## Componentes

### C1 — `internal/core/` (novo, conteúdo movido do root)

```
git mv column_type.go internal/core/column_type.go
git mv conn.go        internal/core/conn.go
git mv connector.go   internal/core/connector.go
git mv data_source.go internal/core/data_source.go
git mv dialect.go     internal/core/dialect.go
git mv errors.go      internal/core/errors.go
git mv logger.go      internal/core/logger.go
git mv options.go     internal/core/options.go
git mv parser.go      internal/core/parser.go        # se M23 já foi implementado neste ponto
# + todos os *_test.go correspondentes
```

`package golem` vira `package core` em cada arquivo movido. Nenhuma outra mudança de conteúdo —
os tipos/funcs continuam com o mesmo nome (`Conn`, `Dialect`, `NewDataSource`, etc.), só o pacote
que os hospeda muda.

**Aceitação:** `go build ./internal/core/...` isolado (sem o resto do módulo) compila.

---

### C2 — Mover os 6 pacotes públicos, trocar import do root por `internal/core`

```
git mv entity      internal/entity
git mv repository  internal/repository
git mv relation    internal/relation
git mv op          internal/op
git mv query       internal/query
git mv join        internal/join
```

Dentro de `internal/entity/*.go` e `internal/repository/*.go` (únicos com a dependência hoje):
`import "github.com/leandroluk/golem"` → `import core "github.com/leandroluk/golem/internal/core"`,
e todo uso de `golem.X` → `core.X` no corpo do arquivo. `relation`/`op`/`query` não mudam import
nenhum (não dependiam do root). `join` só ajusta o path de `entity`/`query` pra `internal/...`.

**Aceitação:** `go vet ./internal/...` limpo. Nenhum arquivo dentro de `internal/` importa o pacote
root `github.com/leandroluk/golem` (só o inverso é permitido).

---

### C3 — `golem.go` (root) — reexport completo

Reescreve do zero, no mesmo espírito do `gonest.go`:

```go
// Package golem is the public entry point of the module. Per the same
// convention gonest.dev/gonest uses (its own AD-004): Go's internal/ path
// rule blocks any package outside this module from importing internal/*,
// so this file is the ONLY door external consumers have -- everything
// below is a type alias or a var-aliased/wrapped function pointing at the
// real implementation living under internal/<concept>. All real logic
// lives in internal/*; this file carries none of its own.
package golem

import (
	"reflect"

	"github.com/leandroluk/golem/internal/core"
	"github.com/leandroluk/golem/internal/entity"
	"github.com/leandroluk/golem/internal/join"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
	"github.com/leandroluk/golem/internal/relation"
	"github.com/leandroluk/golem/internal/repository"
)

// ---- internal/core ----

type Conn = core.Conn
type Tx = core.Tx
type TxConn = core.TxConn
type Result = core.Result
type DataSource = core.DataSource
type Option = core.Option
type Dialect = core.Dialect
type Connector = core.Connector
type ColumnType = core.ColumnType
type Parser = core.Parser
type Logger = core.Logger
type LogLevel = core.LogLevel

var (
	NewDataSource     = core.NewDataSource
	MustNewDataSource = core.MustNewDataSource
	GetDataSource      = core.GetDataSource
	NewTx              = core.NewTx
	WithConnector      = core.WithConnector
	DataSourceName     = core.DataSourceName
	CustomParser       = core.CustomParser
	DefaultParser      = core.DefaultParser
	DefaultLogger      = core.DefaultLogger
	BOOLEAN, SMALLINT, INTEGER, BIGINT, DECIMAL,
	FLOAT, CHAR, VARCHAR, TEXT, DATE, DATETIME,
	TIME, BLOB, UUID, JSON = core.BOOLEAN, core.SMALLINT, core.INTEGER, core.BIGINT, core.DECIMAL,
		core.FLOAT, core.CHAR, core.VARCHAR, core.TEXT, core.DATE, core.DATETIME,
		core.TIME, core.BLOB, core.UUID, core.JSON

	ErrNotFound             = core.ErrNotFound
	ErrDuplicateKey         = core.ErrDuplicateKey
	ErrForeignKeyViolation  = core.ErrForeignKeyViolation
	ErrDataSourceNotFound   = core.ErrDataSourceNotFound
)

// ---- internal/entity ----

type Entity[T any] = entity.Entity[T]
type Table = entity.Table
type Column = entity.Column
type ColumnMeta = entity.ColumnMeta
type ForeignKeyMeta = entity.ForeignKeyMeta
type IndexMeta = entity.IndexMeta
type EntityMeta = entity.EntityMeta
type Index = entity.Index
type FKRegistration = entity.FKRegistration
type HookBuilder[T any] = entity.HookBuilder[T]

var ResolveField = entity.ResolveField
var ForeignKeysReferencing = entity.ForeignKeysReferencing

func NewTable[T any](fn func(t *T, b *Table)) *Entity[T] { return entity.New(fn) }
func AddHook[T any](e *Entity[T]) *HookBuilder[T]         { return entity.AddHook(e) }

// ---- internal/repository ----

type Repository[T any] = repository.Repository[T]

func NewRepository[T any](conn Conn, e *Entity[T]) *Repository[T] {
	return repository.Get(conn, e)
}
func RunAggregate[T, R any](ctx context.Context, r *Repository[T], fn func(t *T, res *R, a *Aggregate[T, R])) ([]R, error) {
	return repository.Aggregate(ctx, r, fn)
}
func Preload[T, J any](ctx context.Context, r *Repository[T], items []T, target *Entity[J], criteria ...func(*J, *Query[J])) (map[any][]J, error) {
	return repository.Preload(ctx, r, items, target, criteria...)
}

// ---- internal/relation ----

type OnDeleteAction = relation.OnDeleteAction
type ForeignKeyOptions = relation.ForeignKeyOptions

var NewForeignKeyOptions = relation.NewForeignKeyOptions

// ---- internal/op ----

type Condition = op.Condition
type Order = op.Order

var (
	Eq, Gt, Gte, Lt, Lte, In, Like = op.Eq, op.Gt, op.Gte, op.Lt, op.Lte, op.In, op.Like
	Or, Not                        = op.Or, op.Not
	Asc, Desc                      = op.Asc, op.Desc
)

// ---- internal/query ----

type AggMapping = query.AggMapping
type Aggregate[T, R any] = query.Aggregate[T, R]
type LockStrength = query.LockStrength
type LockWait = query.LockWait
type Query[T any] = query.Query[T]
type SetClause = query.SetClause
type Update[T any] = query.Update[T]
type Count[T any] = query.Count[T]
type JoinOn = query.JoinOn
type JoinData = query.JoinData
type Join[T any] = query.Join[T]

func NewAggregate[T, R any]() *Aggregate[T, R] { return query.NewAggregate[T, R]() }
func NewQuery[T any]() *Query[T]               { return query.New[T]() }
func NewUpdate[T any]() *Update[T]             { return query.NewUpdate[T]() }
func NewCount[T any]() *Count[T]               { return query.NewCount[T]() }
func NewJoin[T any]() *Join[T]                 { return query.NewJoin[T]() }

// ---- internal/join ----

func JoinInner[T, J any](q *Query[T], target *Entity[J], fn func(j *J, q1 *Join[J])) {
	join.Inner(q, target, fn)
}
func JoinLeft[T, J any](q *Query[T], target *Entity[J], fn func(j *J, q1 *Join[J])) {
	join.Left(q, target, fn)
}
func JoinRight[T, J any](q *Query[T], target *Entity[J], fn func(j *J, q1 *Join[J])) {
	join.Right(q, target, fn)
}
func JoinFull[T, J any](q *Query[T], target *Entity[J], fn func(j *J, q1 *Join[J])) {
	join.Full(q, target, fn)
}
```

Notas de implementação:
- Tipo genérico usa `type X[T any] = pkg.X[T]` (alias genérico — suportado desde Go 1.24; `go.mod`
  já declara `go 1.25.7`, confirmado suficiente, mas **conferir na hora de implementar** que a
  sintaxe compila igual em todos os 5 `driver/*` (mesma versão de Go do módulo raiz).
- `var (A, B = pkg.A, pkg.B)` (var-alias múltiplo numa linha) é só açúcar sintático — cada
  identificador continua individualmente substituível/documentável; usar bloco `var (...)` com 1
  linha por grupo semântico (como no rascunho acima) só por legibilidade, não é regra da linguagem.
- Funções genéricas (`NewTable`, `NewRepository`, `RunAggregate`, `Preload`, `NewAggregate`,
  `NewQuery`, `NewUpdate`, `NewCount`, `NewJoin`, `JoinInner/Left/Right/Full`) precisam ser função
  wrapper de verdade (delegando por chamada), não `var`-alias — mesma exceção forçada pela
  linguagem já documentada no `gonest.go` pra `Parse[T]`/`MustParse[T]`.

**Aceitação:** `go build ./...` verde, `go doc golem` lista tudo (ver Success Criteria do spec.md).

---

### C4 — `driver/*` — confirmar zero mudança

`driver/postgres/*.go` (e os outros 4) continuam `import "github.com/leandroluk/golem"`, usando
`golem.Dialect`, `golem.Option`, `golem.ColumnType` etc. — como esses viram ALIAS de tipo
(`type Dialect = core.Dialect`), não tipo novo, o código de `driver/*` compila sem NENHUMA edição.
**Verificar isso na prática** (não assumir) rodando `go build ./driver/...` logo depois de C3, antes
de mexer em qualquer arquivo dentro de `driver/`.

**Aceitação:** zero diff em `driver/*` (fora dos `dialect_test.go` que já mudam por causa do
`NewTx` do M23, se ainda não tiver sido feito).

---

### C5 — Testes, README, docs, exemplos

- Todo `_test.go` dos 6 pacotes migra junto (C2), sem reescrever asserção — só ajustar import path
  se referenciava outro dos 6.
- `.examples/*` (`blog-api`, `blog-graphql`, `simple-todo`, `lifecycle-hooks`, `config-dotenv` —
  qualquer um que importe `entity`/`repository`/`relation`/`op`/`query`/`join` hoje) trocam pro
  símbolo `golem.X` equivalente.
- `README.md` + `docs/guides/*.md`: todo trecho de código que hoje mostra
  `import "github.com/leandroluk/golem/entity"` (etc.) passa a mostrar só `import "github.com/leandroluk/golem"`,
  com os símbolos usados como `golem.X`.

**Aceitação:** critério do spec.md — `grep` não acha import direto dos 6 pacotes fora de `internal/`.

---

## Arquivos Afetados

| De                                                                                                                                          | Para                              |
| ------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------- |
| `column_type.go`, `conn.go`, `connector.go`, `data_source.go`, `dialect.go`, `errors.go`, `logger.go`, `options.go`, `parser.go` (+ testes) | `internal/core/*`                 |
| `entity/*`                                                                                                                                  | `internal/entity/*`               |
| `repository/*`                                                                                                                              | `internal/repository/*`           |
| `relation/*`                                                                                                                                | `internal/relation/*`             |
| `op/*`                                                                                                                                      | `internal/op/*`                   |
| `query/*`                                                                                                                                   | `internal/query/*`                |
| `join/*`                                                                                                                                    | `internal/join/*`                 |
| `golem.go`                                                                                                                                  | reescrito do zero (reexport puro) |
| `driver/*`                                                                                                                                  | inalterado (só verificado)        |
| `.examples/*`, `README.md`, `docs/guides/*.md`                                                                                              | imports/snippets atualizados      |

---

## Riscos e Mitigações

| Risco                                                                                                                | Mitigação                                                                                                                                      |
| -------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Esquecer algum símbolo exportado na hora de reexportar (regressão silenciosa de API)                                 | Checklist = a tabela completa do `spec.md` REQ-003 + tudo que já era público no root — cruzar linha por linha antes de considerar C3 pronto.   |
| `go doc` de tipo genérico alias (`type X[T any] = pkg.X[T]`) ter comportamento/formatação diferente do tipo original | Verificar na prática (`go doc golem.Entity`) depois de implementar, não assumir que fica idêntico.                                             |
| Alias genérico exigir versão de Go mais nova que a que os 5 `driver/*`/consumidores usam                             | Confirmar `go.mod`'s `go 1.25.7` é consistente em todo o módulo antes de começar (já é um módulo único, mas confirmar mesmo).                  |
| `internal/core` acabar precisando de algo de `internal/entity`/etc. no futuro (tentação de import de volta)          | Regra dura: se acontecer, é sinal de que a coisa não deveria estar em `internal/core` — mover pro pacote-folha certo, nunca importar de volta. |

---

## Ordem de Implementação Sugerida

1. C1 (`internal/core`) — isolado, testável sozinho.
2. C2 (mover os 6 pacotes + trocar import por `internal/core`) — depende de C1 existir.
3. C3 (`golem.go` reexport) — depende de C1+C2 (precisa de todos os pacotes internos no lugar final).
4. C4 (verificar `driver/*`) — depende de C3 estar compilando.
5. C5 (testes/README/docs/exemplos) — último, depois que a API estiver estável e compilando.

Pré-requisito de ordem MAIOR: só começar M24 depois que M23 (`custom-field-parser`) tiver fechado
— `parser.go` precisa existir e já estar testado antes de mover pra `internal/core` em C1, senão os
dois trabalhos colidem no mesmo arquivo em paralelo.
