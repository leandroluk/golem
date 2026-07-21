# Tasks: M23 — Custom Field Parser

**Última atualização:** 2026-07-20
**Status global:** Done

---

## Sumário de Dependências

```
T1 (golem/parser.go: Parser, defaultParser, duck-typing)
  ├─ T2 (options.go: CustomParser + data_source.go: DataSource.parser)
  │    └─ T3 (conn.go: Conn.Parser() + NewTx ganha parser) ── depende de T1 e T2
  │         ├─ T4 (internal/scanner: Plan.parser, Compile assina parser, assignReflect delega)
  │         │    └─ T5 (repository: toDriverValue morre, usa conn.Parser(); Get[T] passa parser pro scanner.Compile) ── depende de T3 e T4
  │         └─ T6 (5× driver/*/dialect_test.go: NewTx ganha 3º argumento) [P com T4/T5]
  │
  └─ T7 (README + docs/guides/schema.md) ── depende de T5 e T6 (API estável)

T8 (verificação final: build/vet/test -race completo + confirma testes do AD-055 sem alteração de asserção) ── depende de T5, T6, T7
```

---

## Tarefas

### T1 — `golem/parser.go`: `Parser`, `DefaultParser`, `toSQLValue`/`fromSQLValue`

**O quê:** Novo arquivo no pacote root. Interface `Parser` (`ToSQL`/`FromSQL`), implementação
`defaultParser` (superset da lógica hoje em `repository.toDriverValue`/`scanner.scanInto` do
AD-055, MAIS o passo de duck-typing `Get()`/`Set(T)` por nome de método via reflection — ver
design.md C1 pro código de referência completo).

**Onde:**
- [NEW] `golem/parser.go` — `Parser`, `defaultParser`, `DefaultParser`, `toSQLValue`, `fromSQLValue`, `sqlScanner` (interface local, evita depender de `database/sql` só por essa assinatura — **conferir na hora**: se `golem.go`/outro arquivo do root já importa `database/sql`, usar `sql.Scanner` direto em vez de redeclarar)
- [NEW] `golem/parser_test.go` — testes diretos de `defaultParser`, cobrindo TODOS os passos de resolução (`driver.Valuer` addr/valor, duck-type `Get()`, deref ponteiro, passthrough nativo, enum→kind, fallback JSON — write; `sql.Scanner`, duck-type `Get()+Set(T)`, nil→zero, ponteiro aloca, match/convertible, JSON→struct — read), incluindo um tipo fake com o mesmo shape de `gonest.Accessor[T]` (sem importar `gonest` — `golem` não pode depender dele) pra provar o duck-typing funciona pra `T` arbitrário

**Reutiliza:** lógica já escrita e testada em `repository.toDriverValue`/`scanner.assignReflect`
(AD-055) — migra, não reescreve do zero.

**Done when:**
- `Parser`/`DefaultParser` exportados e documentados (doc comment explica a ordem de resolução)
- Teste do tipo fake `Get()/Set(T)` prova ida E volta (`ToSQL` desembrulha, `FromSQL` embrulha de volta)
- `go test ./... -run TestParser -cover` (ou nome equivalente do arquivo) cobre 100% de `parser.go`

**Gate:** `go test ./... -run TestParser -cover`

---

### T2 — `options.go`: `CustomParser` + `data_source.go`: `DataSource.parser`

**O quê:** `dataSourceConfig` ganha campo `parser Parser`. Nova `Option` `CustomParser(fn func(Parser) Parser) Option`
que chama `fn(DefaultParser)` e guarda o resultado. `NewDataSource` resolve `cfg.parser` pra
`DefaultParser` se nunca setado. `DataSource` struct ganha campo `parser Parser` (copiado de
`cfg.parser` na construção).

**Onde:**
- [MODIFY] `options.go` — `dataSourceConfig.parser`, `func CustomParser(...) Option`
- [MODIFY] `data_source.go` — `DataSource.parser` campo; `NewDataSource`: default pra `DefaultParser` se `cfg.parser == nil`
- [MODIFY] `data_source_test.go` — teste provando `CustomParser` troca o parser ativo, e que omitir a Option resolve pra `DefaultParser`

**Depende de:** T1 (precisa do tipo `Parser`/`DefaultParser`)

**Done when:**
- `golem.NewDataSource(postgres.New(...), golem.CustomParser(func(base Parser) Parser {...}))` compila e usa o parser customizado
- Omitir `CustomParser` resolve pra `DefaultParser` (teste explícito)

**Gate:** `go test . -run TestNewDataSource -cover`

---

### T3 — `conn.go`: `Conn.Parser()` + `NewTx` ganha `parser` (breaking)

**O quê:** `Conn` (interface pública) ganha `Parser() Parser`. `DataSource.Parser()` retorna
`ds.parser`. `txImpl` ganha campo `parser Parser` + método `Parser()`. `NewTx(dialect, txConn)` →
`NewTx(dialect, txConn, parser)`. Único call site de produção (`DataSource.Transaction`, em
`data_source.go`) passa `ds.parser`.

**Onde:**
- [MODIFY] `conn.go` — `Conn` interface: `+Parser() Parser`; `txImpl.parser` campo + método; `NewTx` assinatura
- [MODIFY] `data_source.go` — `Transaction(ctx, fn)`: `NewTx(dialect, txConn, ds.parser)`
- [MODIFY] `conn_test.go` — `var _ Conn = (*txImpl)(nil)` (guard já deve existir pra `DataSource`, conferir se existe equivalente pra `txImpl`, criar se não); teste provando `conn.Parser()` dentro de uma `Transaction` devolve o mesmo valor de `dataSource.Parser()` fora dela

**Depende de:** T1, T2 (precisa de `DataSource.parser` já existindo pra propagar)

**Done when:**
- `go build ./...` COM ERRO esperado nos 5 `driver/*/dialect_test.go` (só eles chamam `NewTx` direto) — confirma que a mudança é breaking exatamente onde o design previu, nada além disso
- `var _ Conn = (*DataSource)(nil)` e `var _ Conn = (*txImpl)(nil)` compilam

**Gate:** `go build .` (pacote root isolado, sem `./...` ainda — os 5 adapters só ficam verdes depois de T6)

---

### T4 — `internal/scanner`: `Plan.parser`, `Compile` ganha parâmetro, `assignReflect` delega

**O quê:** `Plan` struct ganha campo `parser golem.Parser`. `Compile(meta entity.EntityMeta, parser golem.Parser) *Plan`
guarda o parser recebido. `assignReflect` (fallback `kindOther`) troca a lógica hardcoded (AD-055)
por `parser.FromSQL(field, raw)` — a lógica em si já migrou pro `defaultParser.FromSQL` em T1, aqui
só troca quem chama.

**Onde:**
- [MODIFY] `internal/scanner/scanner.go` — `Plan.parser`, `Compile` assinatura, `assignFast`/`assignReflect` recebem/repassam `parser`
- [MODIFY] `internal/scanner/scanner_test.go` — todo `Compile(meta)` vira `Compile(meta, golem.DefaultParser)`; `TestScanFromMap_SqlScanner_*` (AD-055) continuam SEM alteração de asserção

**Verificar antes de implementar:** `internal/scanner` importar `golem` (root) pra referenciar o
tipo `golem.Parser` — confirmar que isso NÃO cria ciclo (`golem.go` não importa `internal/scanner`
hoje, nem vai importar nesta feature — só M24 mexeria nisso, e nem lá `internal/scanner` fica do
lado que o root importa de volta). Rodar `go build ./internal/scanner/...` isolado antes de seguir
pra T5 se houver qualquer dúvida.

**Depende de:** T1 (tipo `golem.Parser`)

**Done when:**
- `TestScanFromMap_SqlScanner_DelegatesToScan`/`PropagatesScanError` (AD-055, já existentes) passam sem mudança de asserção
- Teste novo prova duck-typing `Get()/Set(T)` funcionando no caminho de leitura (mesmo tipo fake de T1, ou um específico de scanner se mais simples)
- `go test ./internal/scanner/... -cover` = 100%

**Gate:** `go test ./internal/scanner/... -cover`

---

### T5 — `repository`: `toDriverValue` morre, usa `conn.Parser()`; `Get[T]` passa parser pro scanner

**O quê:** Remove `toDriverValue` (helper do AD-055). Os 6 call sites (`Insert`, `SaveOne` ×2,
`Delete` ×2, `Restore`) chamam `r.conn.Parser().ToSQL(fieldVal)` direto. `Get[T]` chama
`scanner.Compile(meta, conn.Parser())` em vez de `scanner.Compile(meta)`.

**Onde:**
- [MODIFY] `repository/repository.go` — remove `toDriverValue`; 6 call sites; `Get[T]`
- [MODIFY] `repository/repository_test.go` — `TestRepository_Insert_ValuerField_*`/`ValuerFieldError_*` (AD-055) continuam SEM alteração de asserção

**Depende de:** T3 (precisa de `conn.Parser()`), T4 (precisa da nova assinatura de `scanner.Compile`)

**Done when:**
- Testes do AD-055 em `repository_test.go` passam sem alteração de asserção
- `go test ./repository/... -cover` = 100%

**Gate:** `go test ./repository/... -cover`

---

### T6 — 5× `driver/*/dialect_test.go`: `NewTx` ganha 3º argumento [P]

**O quê:** Toda chamada `golem.NewTx(dialect, txConn)` nos 5 arquivos de teste vira
`golem.NewTx(dialect, txConn, golem.DefaultParser)` — mecânico, nenhum desses testes exercita
comportamento de parser, só topologia de transação.

**Onde:**
- [MODIFY] `driver/postgres/dialect_test.go`
- [MODIFY] `driver/mysql/dialect_test.go`
- [MODIFY] `driver/mssql/dialect_test.go`
- [MODIFY] `driver/sqlite/dialect_test.go`
- [MODIFY] `driver/oracle/dialect_test.go`

**Depende de:** T3 (assinatura nova de `NewTx`)
**Paralelizável com:** T4, T5 (arquivos completamente independentes, mesmo nível de dependência — só de T3)

**Done when:**
- `go build ./driver/...` verde nos 5 adapters
- `go test ./driver/... -short` verde (sem precisar de Docker — mesma convenção já usada no repo)

**Gate:** `go build ./driver/...`

---

### T7 — README + `docs/guides/schema.md`

**O quê:** Seção "Custom field types (Valuer/Scanner)" (introduzida no AD-055) é substituída por
uma seção "Custom Parser" documentando `golem.Parser`/`golem.CustomParser`, com o exemplo do
`Money`/`Order` do design.md adaptado (ou um exemplo de duck-typing `Get()/Set(T)`, mais fiel ao
que a feature realmente resolve).

**Onde:**
- [MODIFY] `README.md`
- [MODIFY] `docs/guides/schema.md`

**Depende de:** T5, T6 (API precisa estar estável/compilando antes de documentar)

**Done when:**
- Exemplo de código no README compila mentalmente contra a API final (conferir símbolo por símbolo, não só copiar do design.md sem revisar)
- Nenhuma referência residual a `Column[T]`/`driver.Valuer`/`sql.Scanner` como "o jeito de fazer isso" sem mencionar que `Parser` é o ponto de extensão atual

**Gate:** nenhum (documentação, não código) — revisão manual

---

### T8 — Verificação final

**O quê:** Roda a suíte inteira, confirma que M23 fechou sem regressão em nada que já existia.

**Onde:** nenhum arquivo novo — só execução.

**Depende de:** T5, T6, T7

**Done when:**
- `go build ./...` verde
- `go vet ./...` limpo
- `go test ./... -race` verde, TODOS os pacotes + os 5 adapters
- `task coverage` (ou comando equivalente do `Taskfile.yml`) sem regressão de cobertura
- `STATE.md` ganha AD novo documentando M23 fechado (mesmo padrão de AD-053 pro M22)
- `ROADMAP.md`: M23 marcado `✅ DONE`

**Gate:** `go test ./... -race` + `task coverage`

---

## Checklist de Execução

- [x] T1 — `golem/parser.go`
- [x] T2 — `CustomParser` + `DataSource.parser`
- [x] T3 — `Conn.Parser()` + `NewTx` breaking
- [x] T4 — `internal/scanner` integra `Parser`
- [x] T5 — `repository` integra `Parser`
- [x] T6 — 5× `dialect_test.go` mecânico [P]
- [x] T7 — README + docs
- [x] T8 — verificação final + STATE.md/ROADMAP.md
