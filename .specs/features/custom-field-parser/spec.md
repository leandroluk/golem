# Spec: Custom Field Parser (M23)

**ID:** M23
**Scope:** Large
**Status:** ✅ DONE (AD-057) — tag `v0.25.0`
**Breaking:** Sim — muda `golem.Conn` (interface pública) e `golem.NewTx` (assinatura pública)
**Versão alvo:** `v0.25.0` (ver AD-056)

---

## Contexto

AD-055 (2026-07-20) já resolveu parte do problema: um campo de struct cujo tipo implementa
`database/sql/driver.Valuer`/`sql.Scanner` tem prioridade sobre o caminho de reflect padrão, tanto
em `repository.toDriverValue` (write) quanto em `scanner.assignReflect` (read). Isso deixou de
funcionar bem no uso real: o projeto consumidor (`erc`, `ctrl/api`) tem um tipo de dirty-tracking
pra campos de entidade (`gonest.Accessor[T]`, do framework `gonest` — não relacionado ao `golem`,
não pode ganhar método nenhum, é escopo fora de banco de dados) e precisava criar um tipo-wrapper
extra (`column.Column[T]`, embutindo `Accessor[T]` só pra ganhar `Value()`/`Scan()`) pra caber no
mecanismo do AD-055. Isso duplicou a árvore de structs de entidade (domínio com `Accessor[T]`,
tabela com `Column[T]`) só por causa de uma limitação de detecção do `golem`.

A limitação de fundo: Go não tem como um type-switch estático (`case Accessor[any]:`) bater com
`Accessor[*string]`, `Accessor[bool]` etc. — são tipos concretos distintos por instanciação de
generic. E como `Accessor[T]` não pode ganhar `Value()`/`Scan()` (fora do escopo do `gonest`), a
única forma de detectar dinamicamente "isso é um wrapper de leitura/escrita, T qualquer" é via
**duck-typing por reflection** (achar método por NOME — `Get`/`Set` — não por interface fixa),
algo que só faz sentido morar num ponto de extensão CENTRALIZADO e configurável, não espalhado
por tipo. Daí a proposta: um `golem.Parser` plugável por `DataSource`, com implementação padrão
(cobre tudo que o AD-055 já cobre, mais o duck-typing de `Get()`/`Set()`), substituível/decorável
via `golem.CustomParser(fn func(Parser) Parser) Option` passado pro `NewDataSource`.

Isso elimina de vez a necessidade do `column.Column[T]` no lado do consumidor: `gonest.Accessor[T]`
(ou qualquer outro wrapper de terceiros com o mesmo shape `Get()/Set(T)`) passa a funcionar como
campo de entidade `golem` direto, sem tipo intermediário.

---

## Problema a Resolver

### REQ-001 — `golem.Parser`: interface pública de conversão Go ↔ `driver.Value`

Nova interface em `golem` (novo arquivo `parser.go`):

```go
type Parser interface {
	ToSQL(fieldVal reflect.Value) (driver.Value, error)
	FromSQL(dst reflect.Value, raw any) error
}
```

`ToSQL` recebe o `reflect.Value` do campo (endereçável, mesma forma que `repository.toDriverValue`
recebe hoje) e devolve o `driver.Value` a bindar. `FromSQL` recebe `dst` (endereçável, já do tipo
exato do campo — mesma forma que `scanner.assignReflect` já resolve) e o valor cru lido da linha,
escrevendo direto em `dst`.

**Aceitação:** `golem.Parser` é exportado, `golem.DefaultParser` é uma instância pronta de uso
(implementação padrão, ver REQ-002), e ambos aparecem em `go doc golem`.

---

### REQ-002 — `defaultParser`: mesma cobertura do AD-055 + duck-typing `Get()`/`Set(T)`

A implementação padrão de `Parser` (tipo não-exportado `defaultParser`, instância exportada
`DefaultParser`) precisa, nessa ordem, por chamada:

**`ToSQL(fieldVal reflect.Value) (driver.Value, error)`:**

1. Se `fieldVal.Addr().Interface()` ou `fieldVal.Interface()` implementa `driver.Valuer` → delega
   pra `.Value()` (comportamento do AD-055, preservado).
2. Senão, se `reflect.Value.MethodByName("Get")` existe, tem 0 argumentos e 1 retorno → chama,
   pega o valor desembrulhado, e recursa `ToSQL` nele (cobre `Accessor[T]` — e qualquer outro tipo
   com esse shape — sem `T` fixo, sem interface fixa).
3. Senão, dereferencia ponteiro (`nil` → `nil`, ponteiro não-nil → recursa no valor apontado).
4. Senão, passa direto os tipos que `driver.Value` já aceita nativamente (`string`, `bool`,
   `time.Time`, `[]byte`, inteiros/floats).
5. Senão, reduz tipo nomeado/enum (`type X string`) pro kind subjacente via `reflect.Kind`.
6. Senão, `json.Marshal` como fallback (colunas jsonb: mapas, structs, slices de struct).

**`FromSQL(dst reflect.Value, raw any) error`:**

1. Se `dst.Addr().Interface()` implementa `sql.Scanner` → delega pra `.Scan(raw)` (comportamento
   do AD-055, preservado).
2. Senão, se `dst.Addr()`'s method set expõe `Set` (1 argumento, 0 ou 1 retorno do tipo `error`)
   **e** `Get` (duck-type de "isso é um wrapper") → aloca o tipo esperado pelo parâmetro de `Set`,
   escreve nele recursivamente via `FromSQL`, chama `Set(...)`.
3. Senão, match direto de tipo, ou convertible via `reflect.Type.ConvertibleTo` (numérico
   widening, enum ⇄ string/int subjacente).
4. Senão, `[]byte`/`string` cru → `json.Unmarshal` no tipo de `dst` (jsonb → map/struct).
5. Senão, erro descritivo (`golem: cannot scan %s into %s`).

**Aceitação:** todo teste que hoje cobre `TestScanFromMap_SqlScanner_*`/
`TestRepository_Insert_ValuerField_*` (AD-055) continua passando sem alteração de asserção —
prova que a nova implementação é estritamente um superset da anterior. Testes novos cobrem
especificamente o duck-typing `Get()/Set(T)` pra um tipo fake com o mesmo shape de
`gonest.Accessor[T]` (sem importar `gonest`, golem não pode depender dele).

---

### REQ-003 — `golem.CustomParser`: Option pra sobrescrever/decorar o parser por `DataSource`

Nova `Option` em `options.go`:

```go
func CustomParser(fn func(defaultParser Parser) Parser) Option
```

`fn` recebe `golem.DefaultParser` e devolve o `Parser` que o `DataSource` deve usar — permite tanto
substituição total quanto decoração (guardar `defaultParser` e cair nele como fallback). Se
`CustomParser` nunca for chamado, `DataSource` usa `DefaultParser` sem indireção extra.

**Aceitação:**

```go
dataSource := golem.MustNewDataSource(
	postgres.New(func(o *postgres.Options) { o.DSN = dsn }),
	golem.CustomParser(func(base golem.Parser) golem.Parser {
		return myParser{base: base}
	}),
)
```
resulta em `dataSource.Parser()` (REQ-004) retornando `myParser{...}`, não `DefaultParser`.

---

### REQ-004 — `golem.Conn` ganha `Parser() Parser`

`Conn` (interface pública, `conn.go`) ganha o método `Parser() Parser`. `DataSource` e `txImpl`
(via `Tx`, que embute `Conn`) implementam. Uma transação usa o MESMO parser da `DataSource` que a
originou — não há reconfiguração de parser por transação.

**Aceitação:** `var _ Conn = (*DataSource)(nil)` e `var _ Conn = (*txImpl)(nil)` continuam
compilando (já existem como guard hoje pra `DataSource`; adicionar o equivalente pra `txImpl` se
não existir). Chamar `conn.Parser()` dentro de uma `Transaction(ctx, fn)` devolve o mesmo valor que
`dataSource.Parser()` fora dela.

---

### REQ-005 — `golem.NewTx` recebe o `Parser` (breaking, mecânico)

`NewTx(dialect Dialect, txConn TxConn) Tx` vira `NewTx(dialect Dialect, txConn TxConn, parser Parser) Tx`.
`DataSource.Transaction` (único call site em código de produção) passa `ds.parser`. Os 5
`driver/*/dialect_test.go` que chamam `golem.NewTx` diretamente (só testes, nenhum adapter de
produção chama) precisam do argumento novo — mecânico, sem mudança de comportamento nesses testes.

**Aceitação:** `go build ./...` e `go test ./... -race` verdes nos 5 adapters após o update
mecânico dos testes.

---

### REQ-006 — `repository`/`scanner` passam a usar `Conn.Parser()`/`Plan`-scoped `Parser`

`repository.toDriverValue` (helper do AD-055) some — os 6 call sites (`Insert`, `SaveOne` ×2,
`Delete` ×2, `Restore`) chamam `r.conn.Parser().ToSQL(fieldVal)` direto. `internal/scanner.Compile`
ganha um parâmetro `parser Parser`, guardado no `Plan`; `assignReflect` (fallback `kindOther`) troca
a lógica hardcoded (movida pro `defaultParser` do REQ-002) por `plan.parser.FromSQL(field, raw)`.
`repository.Get[T]` passa `conn.Parser()` pra `scanner.Compile`.

**Aceitação:** toda a suíte `repository`/`internal/scanner` (existente + REQ-002's testes novos)
verde. Nenhuma regressão de allocação no fast path (`assignFast` continua intocado pra os kinds
primitivos — só o fallback muda de dono).

---

## Fora de Escopo

| Item | Motivo |
|---|---|
| Parser por-coluna (`b.Col(&t.X, ...).Parser(myParser)`) | Overengineering pro caso de uso atual (1 parser cobre todos os wrappers do mesmo shape); considerar só se aparecer necessidade real de comportamento por-coluna. |
| Remover `column.Column[T]` do projeto consumidor `erc` | É mudança no repo `erc`, não no `golem` — decisão e execução ficam pro `erc` depois que essa feature for lançada. |
| `Dialect.Bind`/`Scan` | Já removidos (AD-053), não ressuscitam aqui. |
| Parser assíncrono/com `context.Context` | Nenhum caso de uso real pede I/O dentro de `ToSQL`/`FromSQL` — mantém a assinatura síncrona simples. |

---

## Restrições

- Backward-incompatible de propósito (`Conn`/`NewTx`) — aceito, versão bump reflete isso (`v0.25.0`, ver AD-056).
- `golem` continua sem importar/conhecer `gonest` — o duck-typing (REQ-002) é sempre por NOME de método via reflection, nunca por tipo concreto de um pacote externo.
- `assignFast` (path primitivo, zero-alloc) não pode regredir em performance — mudança fica inteiramente no fallback `kindOther`/`toDriverValue`.
- Manter 100% de cobertura (AD-026/precedente do projeto).
- `unsafe` continua só em `internal/`.

---

## Requirement Traceability

| Requirement ID | Fase | Status |
| --- | --- | --- |
| M23-REQ-001 | Execute | Verified |
| M23-REQ-002 | Execute | Verified |
| M23-REQ-003 | Execute | Verified |
| M23-REQ-004 | Execute | Verified |
| M23-REQ-005 | Execute | Verified |
| M23-REQ-006 | Execute | Verified |

**Coverage:** 6 total, 0 mapeados pra tasks ainda, 6 não-mapeados ⚠️

---

## Success Criteria

- [x] `go build ./...` e `go test ./... -race` verdes, todos os pacotes + os 5 adapters.
- [x] Testes do AD-055 (`TestScanFromMap_SqlScanner_*`, `TestRepository_Insert_ValuerField_*`) passam sem alteração de asserção.
- [x] Teste novo prova duck-typing `Get()/Set(T)` funcionando pra um tipo fake com o mesmo shape de `gonest.Accessor[T]`, sem `golem` importar `gonest`.
- [x] README + `docs/guides/schema.md` documentam `Parser`/`CustomParser`, substituindo (não só complementando) a seção "Custom field types (Valuer/Scanner)" do AD-055.
- [x] Tag `v0.25.0` publicada após merge.
