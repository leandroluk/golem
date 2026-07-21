# Design: Custom Field Parser (M23)

**Spec**: `.specs/features/custom-field-parser/spec.md`
**Status**: Draft

---

## Diagnóstico: O Path Atual (AD-055)

```
repository.Insert/SaveOne/Delete/Restore
  → toDriverValue(fieldVal reflect.Value) (driver.Value, error)   [unexported, repository.go]
      → fieldVal.Addr().Interface().(driver.Valuer)  ?
      → fieldVal.Interface().(driver.Valuer)         ?
      → senão: fieldVal.Interface()  (raw, sem conversão)

internal/scanner.assignReflect (fallback kindOther)
  → field.Addr().Interface().(sql.Scanner)  ?
      → senão: match direto / convertible / erro
```

Os dois pontos de extensão (`toDriverValue`, `assignReflect`) são **hardcoded dentro de
`repository`/`internal/scanner`** — não existe um tipo `Parser` nomeado, não há como o consumidor
trocar o comportamento sem reimplementar `driver.Valuer`/`sql.Scanner` no PRÓPRIO tipo de campo.
Isso forçou o consumidor (`erc`) a criar um wrapper (`column.Column[T]`) só pra colar os 2 métodos
em cima de um tipo de terceiros (`gonest.Accessor[T]`) que não pode ganhar método nenhum — a
árvore de entidade dobrou de tamanho (domínio + tabela) por causa disso.

### Decisão Arquitetural

Extrai a lógica de `toDriverValue`/`assignReflect` pra uma interface nomeada e exportada
(`golem.Parser`), com implementação padrão (`defaultParser`, superset do que já existe) e um ponto
de substituição por `DataSource` (`golem.CustomParser`, uma `Option`). O `defaultParser` ganha um
passo NOVO antes do fallback de JSON: duck-typing via `reflect.Value.MethodByName("Get"/"Set")` —
detecta "isso é um wrapper de valor" por FORMATO de método, não por interface fixa nem por tipo
concreto de pacote externo, então funciona pra `gonest.Accessor[T]` (T qualquer) sem `golem`
importar `gonest` e sem `Accessor` implementar nada.

`Conn` (interface pública) ganha `Parser() Parser` pra `repository`/`scanner` alcançarem o parser
ativo a partir de um `golem.Conn` já em mãos — mesmo mecanismo que `Conn.Dialect()` já usa hoje pra
alcançar o `Dialect` ativo.

---

## Componentes

### C1 — `golem/parser.go` (novo arquivo)

```go
package golem

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// Parser converts a struct field's Go value to/from the driver.Value
// database/sql understands. DataSource uses DefaultParser unless overridden
// via CustomParser at NewDataSource time.
type Parser interface {
	// ToSQL converts fieldVal (an addressable reflect.Value pointing at the
	// struct field being written) into a driver.Value to bind.
	ToSQL(fieldVal reflect.Value) (driver.Value, error)
	// FromSQL writes raw (as read off the row) into dst, an addressable
	// reflect.Value already of the field's exact Go type.
	FromSQL(dst reflect.Value, raw any) error
}

type defaultParser struct{}

// DefaultParser is golem's built-in Parser. See ToSQL/FromSQL below for the
// exact resolution order.
var DefaultParser Parser = defaultParser{}

func (defaultParser) ToSQL(fieldVal reflect.Value) (driver.Value, error) {
	return toSQLValue(fieldVal)
}

func (defaultParser) FromSQL(dst reflect.Value, raw any) error {
	return fromSQLValue(dst, raw)
}
```

`toSQLValue`/`fromSQLValue` (unexported funcs, mesmo arquivo) contêm a lógica migrada
de `repository.toDriverValue`/`scanner.scanInto` (AD-055), MAIS o passo de duck-typing:

```go
func toSQLValue(fieldVal reflect.Value) (driver.Value, error) {
	// 1. driver.Valuer (endereço primeiro -- cobre pointer receiver -- depois valor)
	if fieldVal.CanAddr() {
		if v, ok := fieldVal.Addr().Interface().(driver.Valuer); ok {
			return v.Value()
		}
	}
	if v, ok := fieldVal.Interface().(driver.Valuer); ok {
		return v.Value()
	}

	// 2. duck-type Get() T -- acha por NOME, funciona pra qualquer T
	if m := fieldVal.MethodByName("Get"); m.IsValid() && m.Type().NumIn() == 0 && m.Type().NumOut() == 1 {
		inner := m.Call(nil)[0]
		return toSQLValue(inner)
	}
	if fieldVal.CanAddr() {
		if m := fieldVal.Addr().MethodByName("Get"); m.IsValid() && m.Type().NumIn() == 0 && m.Type().NumOut() == 1 {
			inner := m.Call(nil)[0]
			return toSQLValue(inner)
		}
	}

	// 3. deref ponteiro
	if fieldVal.Kind() == reflect.Pointer {
		if fieldVal.IsNil() {
			return nil, nil
		}
		return toSQLValue(fieldVal.Elem())
	}

	// 4. passthrough dos tipos que driver.Value já aceita nativamente
	switch v := fieldVal.Interface().(type) {
	case time.Time:
		return v, nil
	case []byte:
		return v, nil
	case string:
		return v, nil
	case bool:
		return v, nil
	}

	// 5. reduz named/enum type pro kind subjacente
	switch fieldVal.Kind() {
	case reflect.String:
		return fieldVal.String(), nil
	case reflect.Bool:
		return fieldVal.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fieldVal.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(fieldVal.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return fieldVal.Float(), nil
	}

	// 6. fallback JSON (map/struct/slice-de-struct -- colunas jsonb)
	raw, err := json.Marshal(fieldVal.Interface())
	if err != nil {
		return nil, fmt.Errorf("golem: json marshal %s: %w", fieldVal.Type(), err)
	}
	return raw, nil
}
```

```go
func fromSQLValue(dst reflect.Value, raw any) error {
	// 1. sql.Scanner (via endereço -- dst já é endereçável, vem de scanner.assignReflect)
	if dst.CanAddr() {
		if s, ok := dst.Addr().Interface().(sqlScanner); ok {
			return s.Scan(raw)
		}
	}

	// 2. duck-type Set(T) -- exige também Get() (confirma "é um wrapper", não qualquer Set solto)
	if dst.CanAddr() {
		addr := dst.Addr()
		getM := addr.MethodByName("Get")
		setM := addr.MethodByName("Set")
		if getM.IsValid() && setM.IsValid() && setM.Type().NumIn() == 1 {
			innerType := setM.Type().In(0)
			innerPtr := reflect.New(innerType)
			if err := fromSQLValue(innerPtr.Elem(), raw); err != nil {
				return err
			}
			setM.Call([]reflect.Value{innerPtr.Elem()})
			return nil
		}
	}

	// 3. nil -> zero value
	if raw == nil {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}

	// 4. ponteiro: aloca e recursa no pointee
	if dst.Kind() == reflect.Pointer {
		elemPtr := reflect.New(dst.Type().Elem())
		if err := fromSQLValue(elemPtr.Elem(), raw); err != nil {
			return err
		}
		dst.Set(elemPtr)
		return nil
	}

	rawVal := reflect.ValueOf(raw)

	// 5. match direto / convertible (numérico, enum <-> kind subjacente)
	if rawVal.Type() == dst.Type() {
		dst.Set(rawVal)
		return nil
	}
	if rawVal.Type().ConvertibleTo(dst.Type()) {
		dst.Set(rawVal.Convert(dst.Type()))
		return nil
	}

	// 6. JSON (jsonb -> map/struct)
	var jsonRaw []byte
	switch v := raw.(type) {
	case []byte:
		jsonRaw = v
	case string:
		jsonRaw = []byte(v)
	default:
		return fmt.Errorf("golem: cannot scan %T into %s", raw, dst.Type())
	}
	ptr := reflect.New(dst.Type())
	if err := json.Unmarshal(jsonRaw, ptr.Interface()); err != nil {
		return fmt.Errorf("golem: json unmarshal into %s: %w", dst.Type(), err)
	}
	dst.Set(ptr.Elem())
	return nil
}

// sqlScanner mirrors database/sql.Scanner -- redeclared locally so parser.go
// doesn't need to import database/sql just for this one interface (driver
// sub-package already imports database/sql/driver, not database/sql).
type sqlScanner interface {
	Scan(src any) error
}
```

**Nota de implementação:** `database/sql.Scanner` é exatamente `interface { Scan(src any) error }`
— redeclarar localmente (`sqlScanner`) evita puxar `database/sql` inteiro só por causa dessa
assinatura; se `golem` já importa `database/sql` em outro arquivo do pacote root, usar o tipo real
(`sql.Scanner`) em vez de redeclarar. **Verificar antes de implementar** (Knowledge Verification
Chain passo 1 — checar imports existentes de `golem.go`/outros arquivos do pacote root).

---

### C2 — `golem/options.go` — `CustomParser`

```go
type dataSourceConfig struct {
	name      string
	connector Connector
	parser    Parser // novo campo
}

func CustomParser(fn func(defaultParser Parser) Parser) Option {
	return func(cfg *dataSourceConfig) { cfg.parser = fn(DefaultParser) }
}
```

`NewDataSource` (`data_source.go`): se `cfg.parser == nil` depois de aplicar todas as `Option`,
usa `DefaultParser`. `DataSource` struct ganha campo `parser Parser`.

---

### C3 — `golem/conn.go` — `Conn.Parser()`

```go
type Conn interface {
	isConn()
	Dialect() Dialect
	Parser() Parser // novo
	Exec(ctx context.Context, sql string, args ...any) (Result, error)
}

func (ds *DataSource) Parser() Parser { return ds.parser }

type txImpl struct {
	dialect Dialect
	txConn  TxConn
	parser  Parser // novo
}

func (t *txImpl) Parser() Parser { return t.parser }
```

---

### C4 — `golem/conn.go` / `data_source.go` — `NewTx` ganha `parser`

```go
func NewTx(dialect Dialect, txConn TxConn, parser Parser) Tx {
	return &txImpl{dialect: dialect, txConn: txConn, parser: parser}
}
```

Único call site de produção (`DataSource.Transaction`, `data_source.go`) passa `ds.parser`. Os 5
`driver/*/dialect_test.go` que chamam `golem.NewTx` diretamente precisam do 3º argumento — usar
`golem.DefaultParser` (nenhum desses testes exercita comportamento de parser, só topologia de
transação).

---

### C5 — `internal/repository/repository.go` — usa `conn.Parser()`

Remove `toDriverValue` (helper unexported do AD-055). Os 6 call sites que chamavam
`toDriverValue(fieldVal)` passam a chamar `r.conn.Parser().ToSQL(fieldVal)` direto — mesma
assinatura de retorno `(driver.Value, error)`, troca mecânica, mesmo tratamento de erro
(`fmt.Errorf("repository: <op>: column %q: %w", col.Name, err)`) já existente em cada site.

---

### C6 — `internal/scanner` — `Plan` carrega o `Parser`

```go
type Plan struct {
	assignments []fieldAssignment
	targetsPool sync.Pool
	parser      golem.Parser // novo
}

func Compile(meta entity.EntityMeta, parser golem.Parser) *Plan {
	// ... igual hoje, seta p.parser = parser
}
```

`repository.Get[T]` (único call site de `scanner.Compile`) passa `conn.Parser()`:

```go
func Get[T any](conn golem.Conn, e *entity.Entity[T]) *Repository[T] {
	meta := e.Describe()
	return &Repository[T]{
		conn:     conn,
		meta:     meta,
		entity:   e,
		scanPlan: scanner.Compile(meta, conn.Parser()),
	}
}
```

`assignReflect` (fallback `kindOther`) recebe o parser como parâmetro extra (thread through
`assignFast` → `assignReflect`, ou vira método de `*Plan` pra ler `p.parser` sem precisar de
parâmetro extra em toda a cadeia — **decisão de implementação, ver Riscos**):

```go
func assignReflect(fp unsafe.Pointer, goType reflect.Type, raw any, parser golem.Parser) error {
	if goType == nil {
		return nil
	}
	field := reflect.NewAt(goType, fp).Elem()
	return parser.FromSQL(field, raw)
}
```

A lógica hoje hardcoded em `assignReflect` (AD-055) MOVE inteira pro `defaultParser.FromSQL`
(C1) — `assignReflect` vira só uma casca fininha que delega.

**Import cycle a verificar:** `internal/scanner` importaria `golem` (root) pra referenciar o tipo
`golem.Parser` — hoje `internal/scanner` já importa `github.com/leandroluk/golem` (confirmado:
`scanner.go` importa `entity`, que por sua vez... **verificar antes de implementar** se
`internal/scanner` já importa o root `golem` ou só `entity`/`entity` já importa `golem` — se
`golem` (root) vier a importar `internal/scanner` de volta (não importa hoje, mas verificar) isso
seria ciclo. Não deveria haver ciclo: `golem.go` não importa `internal/scanner` nem `internal/repository`
hoje (scanner/repository são pacotes independentes que o CONSUMIDOR importa separadamente, não o
root). Confirmar isso lendo `golem.go`'s imports atuais antes do primeiro commit desta feature.

---

## Arquivos Afetados

| Arquivo | Mudança |
|---|---|
| `golem/parser.go` | **novo** — `Parser`, `DefaultParser`, `toSQLValue`, `fromSQLValue`, `sqlScanner` |
| `golem/parser_test.go` | **novo** — testes diretos de `defaultParser` (superset AD-055 + duck-typing) |
| `golem/options.go` | `+parser` em `dataSourceConfig`, `+CustomParser` |
| `golem/data_source.go` | `DataSource.parser` campo, `NewDataSource` resolve default, `Parser()` método, `Transaction` passa `parser` pro `NewTx` |
| `golem/conn.go` | `Conn` ganha `Parser()`; `txImpl.parser` campo + método; `NewTx` ganha parâmetro |
| `internal/repository/repository.go` (hoje `repository/repository.go`) | remove `toDriverValue`, 6 call sites viram `r.conn.Parser().ToSQL(...)`; `Get[T]` passa `conn.Parser()` pro `scanner.Compile` |
| `internal/scanner/scanner.go` (hoje `internal/scanner/scanner.go`) | `Plan.parser` campo, `Compile` ganha parâmetro, `assignReflect` delega pro parser |
| `internal/scanner/scanner_test.go` | testes do AD-055 (`TestScanFromMap_SqlScanner_*`) continuam, ajustar chamadas de `Compile(meta)` → `Compile(meta, golem.DefaultParser)` |
| `repository/repository_test.go` | idem — `TestRepository_Insert_ValuerField_*` (AD-055) sem alteração de asserção, só plumbing |
| `driver/{postgres,mysql,mssql,sqlite,oracle}/dialect_test.go` | chamadas de `golem.NewTx(dialect, txConn)` → `golem.NewTx(dialect, txConn, golem.DefaultParser)` |
| `README.md` | seção "Custom field types (Valuer/Scanner)" (AD-055) substituída por "Custom Parser" |
| `docs/guides/schema.md` | idem |

**Nota:** este design.md já assume o path `internal/repository`/`internal/scanner` da M24 (Flat
Root API) — mas M23 é a que fecha PRIMEIRO (spec.md do M24 depende do M23 fechado). Se M23 for
implementada antes de M24, os paths reais durante a implementação de M23 são os ATUAIS
(`repository/repository.go`, `internal/scanner/scanner.go` — este já é `internal/`, só
`repository` que ainda não é). A tabela acima usa os nomes finais só como referência; a fase de
Tasks resolve o path exato conforme a ordem real de execução.

---

## Riscos e Mitigações

| Risco | Mitigação |
|---|---|
| Duck-typing por `MethodByName` é mais lento que dispatch de interface estático | Só roda no fallback `kindOther`/quando `driver.Valuer`/`sql.Scanner` não bate — path primitivo (`assignFast`) intocado, mesma garantia de performance do M22. |
| `assignReflect` ganhar parâmetro extra (`parser`) precisa mudar toda a cadeia de chamada (`assignFast` → `assignReflect`) | Mecânico — threading de 1 parâmetro por 2 funções internas, não-exportadas, sem mudança de comportamento externo. |
| Falso positivo do duck-type (`Get()`/`Set(T)` existir por acaso num tipo que não é um wrapper de valor) | Aceito conscientemente — mesma categoria de trade-off que `sql.Scanner`/`driver.Valuer` já têm (qualquer tipo com esse método vira candidato). Documentar claramente no doc comment de `defaultParser`. |
| Import cycle `internal/scanner` ↔ `golem` (root) | Verificar ANTES de implementar (Knowledge Verification Chain) — ver nota em C6. |
| Breaking change real em `Conn`/`NewTx` | Aceito, documentado no spec.md como breaking, versão `v0.25.0` (AD-056) sinaliza isso. |

---

## Ordem de Implementação Sugerida

1. C1 (`golem/parser.go` + testes diretos) — não depende de mais nada, pode ser verificado isolado.
2. C2 (`CustomParser` Option) — depende só de C1.
3. C3 + C4 (`Conn.Parser()`, `NewTx`) — depende de C1/C2, é o breaking change de fato.
4. C5 (`repository`) — depende de C3 (precisa de `conn.Parser()`).
5. C6 (`scanner`) — depende de C1 (usa `golem.Parser` como tipo) e é consumido por C5 (`Get[T]`).
6. Atualização dos 5 `dialect_test.go` — mecânico, pode ser feito em paralelo com qualquer passo acima após C4.
7. README/docs — último passo, depois que a API estiver estável.
