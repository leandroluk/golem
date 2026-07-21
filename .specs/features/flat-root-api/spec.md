# Spec: Flat Root API (M24)

**ID:** M24
**Scope:** Large
**Status:** ✅ DONE (AD-058)
**Breaking:** Sim — remove 6 pacotes públicos (`entity`, `repository`, `relation`, `op`, `query`, `join`) MAIS tudo que hoje já mora direto no root (`Conn`, `Dialect`, `DataSource`, `Option`, `ColumnType`, `Result`, `Tx`, `Parser`/M23, `Logger`, erros) — tudo migra pra `internal/core` + `internal/<pkg>`, `golem.go` (root) vira 100% casca de reexport, sem lógica própria (ver REQ-000)
**Versão alvo:** a definir na fase de Tasks (depende de M23 fechar antes — ambas breaking, faz sentido publicar juntas ou em sequência imediata)

---

## Contexto

Hoje o `golem` segue o idiom padrão de ORM Go: cada concern vive num pacote próprio importável
(`entity`, `repository`, `relation`, `op`, `query`, `join`), e o consumidor importa cada um que
precisa (`import "github.com/leandroluk/golem/entity"` etc.). Isso é o oposto do padrão que o
`gonest` (framework irmão, mesma stack NestJS+TypeORM-like que motivou o `golem`) já usa —
`gonest`'s AD-004 (`.specs/project/STATE.md` do `gonest`) diz: "1 pacote Go por tipo sob
`internal/`, root só reexporta". `gonest.go` é a ÚNICA porta pública — todo o resto é
`internal/<concept>`, e o root só faz alias de tipo (`type Schema = schema.Schema`) ou wrapper de
função (`var NewSchema = schema.New`).

Motivação: paridade de ergonomia com `gonest` (import único, autocomplete de `golem.` mostra tudo,
sem precisar lembrar em qual sub-pacote cada símbolo mora — mesmo motivo do `@nestjs/common` da
stack original que inspirou os dois projetos) e mover a liberdade de refatorar implementação
interna sem quebrar consumidor pra dentro de `internal/`.

**Achado durante a especificação (não previsto na primeira versão desta spec):** `entity` e
`repository` (código de PRODUÇÃO, não só teste) já importam o `golem` root hoje
(`entity/entity.go`, `entity/hook.go`, `entity/table.go`, `repository/repository.go` — usam
`golem.ColumnType`/`golem.Conn`/`golem.Dialect`). Se só os 6 pacotes movessem pra `internal/` e o
`golem.go` continuasse com `Conn`/`Dialect`/etc. definidos nele mesmo, reexportar os tipos de
`internal/entity`/`internal/repository` criaria um CICLO de import (`internal/entity` → `golem` →
`internal/entity`). Resolução: tudo que hoje mora direto no pacote root (`conn.go`, `dialect.go`,
`data_source.go`, `options.go`, `column_type.go`, `connector.go`, `logger.go`, `errors.go`, e
`parser.go` do M23) MUDA DE CASA pra um pacote-folha novo, `internal/core` — assim `golem.go`
(reexport) E `internal/entity`/`internal/repository`/etc. importam `internal/core` igualmente, sem
nenhum dos dois ser dependência do outro. Mesma classe de problema que o `gonest` já resolveu mais
de uma vez (`internal/appoptions`, `internal/adapter/fiber` ↔ `internal/app` — ver `STATE.md` do
`gonest`), mesma solução.

**Fora dessa migração, por decisão explícita:** `driver/postgres`, `driver/mysql`, `driver/mssql`,
`driver/sqlite`, `driver/oracle` continuam pacotes públicos próprios. Cada um carrega uma
dependência de terceiro pesada e exclusiva (`pgx`, `go-sql-driver/mysql`, `go-mssqldb`,
`modernc.org/sqlite`, `go-ora`) — se os 5 fossem reexportados todos no mesmo `golem.go`, importar
`golem` sozinho passaria a puxar as 5 dependências de driver pra QUALQUER consumidor, mesmo quem só
usa 1 banco. Isso quebraria o isolamento que hoje é proposital (mesmo motivo do `database/sql` +
driver por import separado). `driver/*` fica exatamente como está.

---

## Problema a Resolver

### REQ-000 — Mover o conteúdo atual do root pra `internal/core` (evita ciclo)

`column_type.go`, `conn.go`, `connector.go`, `data_source.go`, `dialect.go`, `errors.go`,
`logger.go`, `options.go` e `parser.go` (novo, M23) migram pra `internal/core/` — mesmos nomes de
arquivo, package muda de `golem` pra `core`. `golem.go` (root) passa a importar
`github.com/leandroluk/golem/internal/core` e reexportar tudo de lá (aliases de tipo + var-alias
de função), exatamente como já reexportará `internal/entity` etc. (REQ-001+).

**Aceitação:** `internal/core` não importa nenhum dos pacotes movidos em REQ-001 (nem
`internal/entity`, nem `internal/repository`, nem os outros 4) — só o inverso é permitido
(`internal/entity`/`internal/repository` importam `internal/core`). `go list -deps ./internal/core`
não deve listar nenhum `internal/entity`/`internal/repository`/etc. na saída.

---

### REQ-001 — Mover os 6 pacotes pra `internal/`

`entity/`, `repository/`, `relation/`, `op/`, `query/`, `join/` (código de produção E de teste, ambos)
migram pra `internal/entity/`, `internal/repository/`, `internal/relation/`, `internal/op/`,
`internal/query/`, `internal/join/`. Import paths internos entre eles (ex: `repository` importa
`entity`/`query`; `join` importa `entity`/`query`) atualizam pro novo path
(`github.com/leandroluk/golem/internal/entity` etc.). Todo import de `"github.com/leandroluk/golem"`
(root) dentro desses 6 pacotes (`entity`, `repository` — ver REQ-000) vira
`"github.com/leandroluk/golem/internal/core"`. Nenhuma lógica muda — é rename de path, não de
comportamento.

**Aceitação:** `go build ./...` falha (esperado, nada reexporta ainda) só por causa de import
quebrado nos pontos que ficam fora de `internal/` (`golem.go`, `driver/*`, exemplos) — nenhum erro
de lógica/teste dentro dos pacotes movidos.

---

### REQ-002 — `golem.go`: reexport de tipos e funções, política de nomes

Pra cada símbolo exportado dos 6 pacotes movidos, `golem.go` (root) ganha:
- **Tipo** → `type X = internal_pkg.X` (alias, não novo tipo — preserva compatibilidade estrutural,
  mesmo padrão que `golem.Dialect`/`golem.ColumnType` já usam hoje).
- **Função/Construtor sem type param** → `var X = internal_pkg.X` (var-aliased, mesmo padrão de
  `WithConnector`/`DataSourceName` hoje).
- **Função/Construtor GENÉRICA (com `[T any]` etc.)** → função wrapper de verdade (não dá pra
  `var`-aliasear função genérica em Go — mesma razão documentada no comentário de `Parse[T]`/
  `MustParse[T]` no `gonest.go`: exceção ao padrão var-alias, forçada pela linguagem).

**Política de nomes** (decidida): nome nu (sem prefixo de origem) por padrão, mudado só quando há
colisão real de identificador (nunca por "achar mais claro"). **Exceção explícita:** o pacote
`join` foge da regra — `Inner`/`Left`/`Right`/`Full` viram `JoinInner`/`JoinLeft`/`JoinRight`/`JoinFull`
mesmo sem colisão, porque sozinhos no root são palavras genéricas demais pra ficar óbvio que são
sobre join (`op`'s `Eq`/`In`/`Or`/`Asc`/`Desc` ficam nus mesmo assim — mais autoexplicativos no
contexto de query condition).

**Aceitação:** toda linha da tabela do REQ-003 vira uma linha em `golem.go`, revisável 1:1 contra
essa tabela.

---

### REQ-003 — Tabela de mapeamento completa

**`entity` → `golem`**

| Hoje (`entity.X`) | Vira (`golem.X`) | Motivo do nome |
|---|---|---|
| `type Entity[T any]` | `type Entity[T any]` | sem colisão |
| `func New[T any](...) *Entity[T]` | `func NewEntity[T any](...) *Entity[T]` | `New` bare colide com `query.New`/`repository.Get` achatados — ver REQ-004 |
| `type Table` | `type Table` | sem colisão |
| `type Column` | `type Column` | sem colisão (`ColumnType` já existe, nome diferente) |
| `type ColumnMeta` | `type ColumnMeta` | sem colisão |
| `type ForeignKeyMeta` | `type ForeignKeyMeta` | sem colisão |
| `type IndexMeta` | `type IndexMeta` | sem colisão |
| `type EntityMeta` | `type EntityMeta` | sem colisão |
| `type Index` | `type Index` | sem colisão |
| `type FKRegistration` | `type FKRegistration` | sem colisão |
| `type HookBuilder[T any]` | `type HookBuilder[T any]` | sem colisão |
| `func ResolveField(...) (string, error)` | `func ResolveField(...) (string, error)` | sem colisão |
| `func ForeignKeysReferencing(...) []FKRegistration` | `func ForeignKeysReferencing(...) []FKRegistration` | sem colisão |
| `func AddHook[T any](e *Entity[T]) *HookBuilder[T]` | `func AddHook[T any](e *Entity[T]) *HookBuilder[T]` | sem colisão |

**`repository` → `golem`**

| Hoje (`repository.X`) | Vira (`golem.X`) | Motivo do nome |
|---|---|---|
| `type Repository[T any]` | `type Repository[T any]` | sem colisão |
| `func Get[T any](conn Conn, e *Entity[T]) *Repository[T]` | `func NewRepository[T any](conn Conn, e *Entity[T]) *Repository[T]` | `Get` bare no root é ambíguo demais (get o quê?) — segue o padrão `New<Coisa>` já estabelecido |
| `func Aggregate[T, R any](ctx, r, fn) ([]R, error)` | `func RunAggregate[T, R any](ctx, r, fn) ([]R, error)` | **colisão real** com `query.Aggregate[T,R]` (tipo) — Go não aceita tipo e função com mesmo nome no mesmo pacote |
| `func Preload[T, J any](...) (map[any][]J, error)` | `func Preload[T, J any](...) (map[any][]J, error)` | sem colisão |

**`relation` → `golem`**

| Hoje (`relation.X`) | Vira (`golem.X`) | Motivo do nome |
|---|---|---|
| `type OnDeleteAction string` | `type OnDeleteAction string` | sem colisão |
| `type ForeignKeyOptions` | `type ForeignKeyOptions` | sem colisão |
| `func NewForeignKeyOptions() *ForeignKeyOptions` | `func NewForeignKeyOptions() *ForeignKeyOptions` | sem colisão |

**`op` → `golem`** (nomes nus, decisão explícita — ver pergunta respondida)

| Hoje (`op.X`) | Vira (`golem.X`) |
|---|---|
| `type Condition` | `type Condition` |
| `func Eq/Gt/Gte/Lt/Lte/In/Like/Or/Not(...)` | `func Eq/Gt/Gte/Lt/Lte/In/Like/Or/Not(...)` |
| `type Order` | `type Order` |
| `func Asc/Desc(fieldPtr any) Order` | `func Asc/Desc(fieldPtr any) Order` |

**`query` → `golem`**

| Hoje (`query.X`) | Vira (`golem.X`) | Motivo do nome |
|---|---|---|
| `type AggMapping` | `type AggMapping` | sem colisão |
| `type Aggregate[T, R any]` | `type Aggregate[T, R any]` | **fica com o nome bare** — a função de mesmo nome em `repository` foi a que precisou renomear (ver acima) |
| `func NewAggregate[T, R any]() *Aggregate[T, R]` | `func NewAggregate[T, R any]() *Aggregate[T, R]` | sem colisão |
| `type LockStrength string` | `type LockStrength string` | sem colisão |
| `type LockWait string` | `type LockWait string` | sem colisão |
| `type Query[T any]` | `type Query[T any]` | sem colisão |
| `func New[T any]() *Query[T]` | `func NewQuery[T any]() *Query[T]` | `New` bare colide com `entity.New` achatado |
| `type SetClause` | `type SetClause` | sem colisão |
| `type Update[T any]` | `type Update[T any]` | sem colisão (é tipo; `Repository[T].Update(...)` é método, namespace diferente) |
| `func NewUpdate[T any]() *Update[T]` | `func NewUpdate[T any]() *Update[T]` | sem colisão |
| `type Count[T any]` | `type Count[T any]` | sem colisão |
| `func NewCount[T any]() *Count[T]` | `func NewCount[T any]() *Count[T]` | sem colisão |
| `type JoinOn` | `type JoinOn` | sem colisão |
| `type JoinData` | `type JoinData` | sem colisão |
| `type Join[T any]` | `type Join[T any]` | sem colisão (funções `Inner`/`Left`/`Right`/`Full` do pacote `join` não usam esse nome) |
| `func NewJoin[T any]() *Join[T]` | `func NewJoin[T any]() *Join[T]` | sem colisão |

**`join` → `golem`** (prefixado `Join*` — decisão explícita, diferente do `op`: `Inner`/`Left`/`Right`/`Full`
sozinhos no root são palavras genéricas demais, sem contexto óbvio de que são join)

| Hoje (`join.X`) | Vira (`golem.X`) |
|---|---|
| `func Inner[T, J any](q *Query[T], target *Entity[J], fn ...)` | `func JoinInner[T, J any](q *Query[T], target *Entity[J], fn ...)` |
| `func Left[T, J any](...)` | `func JoinLeft[T, J any](...)` |
| `func Right[T, J any](...)` | `func JoinRight[T, J any](...)` |
| `func Full[T, J any](...)` | `func JoinFull[T, J any](...)` |

**Aceitação:** `go vet ./...` limpo (nenhum identificador duplicado), `go doc golem` lista todos os
~50 símbolos acima, nenhum a menos.

---

### REQ-004 — `entity.Table`'s callback signature e uso cruzado entre pacotes

`entity.New[T](func(t *T, b *entity.Table) {...})` vira `golem.NewEntity[T](func(t *T, b *golem.Table) {...})`.
Qualquer lugar que hoje importa 2+ dos 6 pacotes pra compor uma chamada (ex: `join.Inner` recebe
`*query.Query[T]` e `*entity.Entity[J]`) continua funcionando igual, porque os tipos internos
(`internal/query.Query[T]`, `internal/entity.Entity[J]`) são os MESMOS por baixo do alias —
`golem.Query[T]` e `internal_query.Query[T]` são identicamente o mesmo tipo em runtime (alias, não
novo tipo), então a assinatura de `golem.Inner[T, J any](q *golem.Query[T], target *golem.Entity[J], fn ...)`
compila sem nenhuma conversão.

**Aceitação:** `golem.JoinInner`/`JoinLeft`/`JoinRight`/`JoinFull` aceitam diretamente o retorno de
`golem.NewQuery[T]()` e `golem.NewEntity[J](...)` sem cast.

---

### REQ-005 — Testes migram junto (sem reescrever), README/docs/exemplos atualizam imports

Todo `_test.go` dos 6 pacotes migra pra dentro de `internal/<pkg>/`, sem mudança de lógica (só
ajuste de import path se referenciavam outro dos 6 pacotes movidos). `README.md`, `docs/guides/*.md`
e `.examples/*` (que hoje fazem `import "github.com/leandroluk/golem/entity"` etc.) atualizam pra
usar só `golem.X` — nenhum exemplo importa mais `entity`/`repository`/`relation`/`op`/`query`/`join`
diretamente depois desta migração (só `golem` + o `driver/*` escolhido).

**Aceitação:** `grep -rn 'golem/entity"\|golem/repository"\|golem/relation"\|golem/op"\|golem/query"\|golem/join"' .`
não retorna nada fora de `internal/` e do próprio `go.mod`/`CHANGELOG` histórico.

---

## Fora de Escopo

| Item | Motivo |
|---|---|
| Mover `driver/*` pra `internal/` | Decisão explícita — ver Contexto. Cada driver carrega dependência de terceiro exclusiva, reexport único forçaria todo consumidor a compilar os 5 juntos. |
| Migrar o projeto consumidor `erc` pros novos nomes (`golem.NewEntity` etc.) | É mudança no repo `erc`, não no `golem` — fica pro `erc` depois que essa release sair. |
| Deprecar com aviso/manter os 6 pacotes antigos como alias temporário | Usuário não pediu transição gradual; é breaking direto, mesmo estilo de M23. |
| Renomear `driver/*`'s próprios símbolos (`postgres.New`, `postgres.Options`) | Só entra em escopo se um dia `driver/*` também for achatado — não é o caso aqui. |

---

## Restrições

- Nenhuma mudança de comportamento — é rename/realocação de símbolo, testes migram sem reescrever asserções.
- Aliases de TIPO (`type X = pkg.X`), nunca tipo novo — preserva compatibilidade estrutural onde faz diferença (ex: `golem.Query[T]` precisa ser o MESMO tipo usado internamente por `Repository[T]`, não um tipo espelho).
- Funções genéricas não podem ser `var`-aliasadas (limitação da linguagem, mesmo caso já documentado em `Parse[T]`/`MustParse[T]` do `gonest`) — viram função wrapper real, ainda fina (delega, sem lógica própria).
- `go vet`/`go build` continuam limpos, cobertura mantida (mesmo padrão de todo milestone anterior).
- Depende de M23 (Custom Field Parser) ter fechado antes — ambas tocam `golem.go`/API pública, evita conflito de merge/numeração de versão indo em paralelo.

---

## Requirement Traceability

| Requirement ID | Fase | Status |
| --- | --- | --- |
| M24-REQ-000 | Execute | Verified |
| M24-REQ-001 | Execute | Verified |
| M24-REQ-002 | Execute | Verified |
| M24-REQ-003 | Execute | Verified |
| M24-REQ-004 | Execute | Verified |
| M24-REQ-005 | Execute | Verified |

**Coverage:** 6 total, 0 mapeados pra tasks ainda, 6 não-mapeados ⚠️

---

## Success Criteria

- [x] `go build ./...` e `go test ./... -race` verdes, todos os pacotes + os 5 adapters.
- [x] `go doc golem` lista todos os ~50 símbolos da tabela do REQ-003 MAIS tudo que já era público no root (`Conn`, `Dialect`, `NewDataSource`, `ColumnType`, `Parser`, etc. — sem regressão nenhuma).
- [x] Nenhum `_test.go`/exemplo/README fora de `internal/` importa `entity`/`repository`/`relation`/`op`/`query`/`join` diretamente.
- [x] `internal/core` não é importado por nenhum dos outros 6 pacotes internos ao contrário (`go list -deps` confirma).
- [x] `driver/postgres` (e os outros 4) continuam pacotes públicos, inalterados nesta migração — nem o import deles muda (continuam `import "github.com/leandroluk/golem"`, que resolve certo via alias).
- [x] Tag publicada após merge, número decidido na fase de Tasks (depende do estado do M23 no momento).
