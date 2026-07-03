# golem

<img align="right" width="180px" src="https://raw.githubusercontent.com/leandroluk/golem/refs/heads/master/assets/golem.png">

[![Build Status](https://github.com/leandroluk/golem/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/leandroluk/golem/actions)  
[![Coverage Status](https://img.shields.io/codecov/c/github/leandroluk/golem/main.svg)](https://codecov.io/gh/leandroluk/golem)  
[![Go Report Card](https://goreportcard.com/badge/github.com/leandroluk/golem)](https://goreportcard.com/report/github.com/leandroluk/golem)  
[![Go Doc](https://godoc.org/github.com/leandroluk/golem?status.svg)](https://pkg.go.dev/github.com/leandroluk/golem)  
[![Release](https://img.shields.io/github/release/leandroluk/golem.svg?style=flat-square)](https://github.com/leandroluk/golem/releases)  

---

## Contents
- [golem](#golem)
  - [Contents](#contents)
  - [Getting started](#getting-started)
  - [Implementation Status](#implementation-status)
  - [About the Project](#about-the-project)
  - [Documentation](#documentation)
    - [Connecting to Postgres](#connecting-to-postgres)
    - [Declaring schemas](#declaring-schemas)
    - [Many-to-many relations (junction entity)](#many-to-many-relations-junction-entity)
    - [Repository (CRUD)](#repository-crud)
    - [Query Builder](#query-builder)
    - [Joins](#joins)
    - [Raw SQL (escape hatch)](#raw-sql-escape-hatch)
    - [Errors](#errors)
    - [Migrations](#migrations)
    - [Custom logger](#custom-logger)
  - [Contributors](#contributors)
  - [License](#license)

---

## Getting started

```go
package main

import (
  golem "github.com/leandroluk/golem"
  postgres "github.com/leandroluk/golem/adapter/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func(o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  if err := dataSource.Connect(); err != nil {
    panic(err)
  }
}
```

See [Documentation](#documentation) below for the full API (entities, repositories, query builder, joins, transactions, raw SQL).

---

## Implementation Status

- [x] M1 - Foundation (`DataSource`, `golem.Conn`, `golem.Dialect`, Postgres adapter)
- [ ] M2 - Schema Declaration (`entity.Builder`, `golem.ColumnType`)
- [ ] M3 - Repository Core CRUD
- [ ] M4 - Query Builder & Read Paths
- [ ] M5 - Update/Count Builders
- [ ] M6 - Joins
- [ ] M7 - Hooks
- [ ] M8 - Transactions
- [ ] M9 - Raw SQL
- [ ] M10 - Typed Errors

See `.specs/project/ROADMAP.md` for the full milestone breakdown.

---

## About the Project

A type-safe, TypeORM-inspired ORM for Go, built with generics and field-pointer references instead of
struct tags or code generation. Entities are declared with plain structs, and every mapping (columns,
keys, relations, hooks, query criteria) is expressed via pointers to struct fields, checked by the Go
compiler — no reflection-by-tag magic, no codegen step.

Multi-dialect ready from day one: entities and column types (`golem.ColumnType`) are dialect-agnostic,
so adding Postgres/MySQL/MSSQL/Oracle later doesn't require rewriting entity declarations. Postgres is
the first adapter (`github.com/leandroluk/golem/adapter/postgres`, via `jackc/pgx/v5`).

Migrations/schema synchronization are explicitly out of scope — entities describe runtime mapping, not
a DDL source of truth. Use an external tool (Liquibase, Flyway, goose, etc.).

See `.specs/project/PROJECT.md` for the full vision/scope and `.specs/project/STATE.md` for the
history of design decisions (AD-001 through AD-017).

---

## Documentation

### Connecting to Postgres

```go
package ex

import (
  golem "github.com/leandroluk/golem"
  postgres "github.com/leandroluk/golem/adapter/postgres"
)

func main() {
  // iniciando uma instância de DataSource nomeada
  dataSource, err := golem.NewDataSource(
    // se não for passada então sempre será "default"
    golem.DataSourceName("example"),
    // passa o conector, obrigatoriamente prcisa ter um conector
    postgres.New(func (o *postgres.Options) {
      // conecta usando o DSN
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"

      // conecta usando propriedades separadas
      o.Host = "localhost"
      o.Port = 5432
      o.User = "postgres"
      o.Password = "1234"
      o.Database = "db"
      o.SSLMode = "disable"

      // define se deve ou não logar as queries sendo executadas
      o.Logging = true
    }),
  )

  defer dataSource.Close()

  if err := dataSource.Connect(); err != nil{
    panic("impossivel conectar ao banco"+err.Error())
  }

  fmt.Println("conectado ao banco de dados")

  // ...
}
```

> **Nota**: Se passar ambos (DSN + propriedades) então propriedades tem prioridade

### Declaring schemas

> `golem.ColumnType` (`golem.BIGINT()`, `golem.VARCHAR(50)`, `golem.TEXT()`, `golem.BOOLEAN()`, `golem.TIMESTAMPTZ()`,
> `golem.UUID()`, `golem.JSON()`, etc.) é agnóstico de dialeto — não vira DDL (esse `golem` não gera schema,
> ver seção Migrations) nem depende de qual adapter você conectou. Serve só de id semântica pro adapter
> saber como fazer bind (Go → driver) e scan (driver → Go) daquele valor, já que `database/sql` sozinho
> não dá conta de tipo exótico (UUID, JSONB, array, ENUM...) de forma consistente entre dialetos. Cada
> adapter (`postgres`, e no futuro `mysql`/`mssql`/etc.) implementa esse contrato:
>
>  type Dialect interface {
>    Bind(t golem.ColumnType, value any) (driver.Value, error)
>    Scan(t golem.ColumnType, raw any, dest any) error
>  }
>
> Isso mantém a entity 100% portável entre dialetos — só o `DataSource`/adapter escolhido em runtime
> decide o dialeto de verdade, a declaração da entity nunca muda.

`entity.Builder` (recebido dentro do callback de `entity.New`) expõe:

escopo:
  tabela:
    - `TableName(name string)`: nome da tabela; padrão = nome da struct (ex: `User` -> `"user"`)
    - `SchemaName(name string)`: schema da tabela; padrão = schema selecionado atualmente na conexão
    - `PrimaryKey(fieldPtrs ...any)`: chave primária; aceita 1+ campos (composta)
    - `Unique(fieldPtrs ...any)`: constraint `UNIQUE`; aceita 1+ campos (composta) — igual `PrimaryKey`/`ForeignKey`, fica fora do `Col` porque unicidade pode envolver mais de uma coluna
    - `Index(fieldPtrs ...any) *index.Builder`: índice secundário sobre 1+ campos
  coluna:
    - `Col(fieldPtr any, type golem.ColumnType) *column.Builder`: mapeia um campo da struct pra uma coluna com tipo explícito
    - `ForeignKey(fieldPtr any, target *entity.Entity[T], opts ...*relation.ForeignKeyOptions)`: chave estrangeira apontando pra PK de outra entity
    - `CreateDate(fieldPtr any) *column.Builder`: marca o campo como "criado em"; preenchido sozinho com o timestamp do insert
    - `UpdateDate(fieldPtr any) *column.Builder`: marca o campo como "atualizado em"; preenchido sozinho com o timestamp de cada update
    - `DeleteDate(fieldPtr any) *column.Builder`: marca o campo como "soft delete em" (presença de valor = registro deletado); `Delete`/`Restore` do `Repository[T]` e todo critério com `Where` passam a filtrar deletados por padrão — ver `.WithDeleted()` na seção Query Builder

`*column.Builder` (retorno de `Col`/`CreateDate`/`UpdateDate`/`DeleteDate`) encadeia:

- `.Name(name string)`: nome da coluna; padrão = nome do campo na struct
- `.Nullable()`: permite `NULL`
- `.Default(value any)`: valor (ou expressão do dialeto) default, vira DDL/constraint no banco
- `.DefaultFunc(fn func() (any, error))`: como `.Default()`, mas o valor é calculado em código (Go) na
  hora do insert em vez de virar expressão no banco — útil pra UUID, slug, valor derivado de outro campo
  etc. só é usado quando o campo estiver zerado; error retornado cancela o insert. (`CreateDate`/
  `UpdateDate` já preenchem o timestamp da operação sozinhos — sem precisar de um `.AutoNow()` à parte,
  já que não faz sentido marcar um campo como "data de criação" e não querer isso)

`*index.Builder` (retorno de `Index`) encadeia:

- `.Name(name string)`: nome do índice; padrão = gerado (`idx_<tabela>_<colunas>`)
- `.Unique()`: índice único

```go
package ex

import (
  "context"

  golem "github.com/leandroluk/golem"
  relation "github.com/leandroluk/golem/relation"
  entity "github.com/leandroluk/golem/entity"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/adapter/postgres"
)

type User struct {
  ID        int64
  CreatedAt time.Time
  UpdatedAt time.Time
  DeletedAt *time.Time
  Name      string
  Email     string
  Age       uint8
}

type Post struct {
  ID          int64
  CreatedAt   time.Time
  UpdatedAt   time.Time
  DeletedAt   *time.Time
  OwnerUserID int64
  Title       string
  Content     string
  Published   bool
}

type Message struct {
  ID           int64
  CreatedAt    time.Time
  UpdatedAt    time.Time
  DeletedAt    *time.Time
  Content      string
  SenderUserID int64
  PostID       int64
}

var UserEntity = entity.New[User](func(t *User, b *entity.Builder) {
  // nome da tabela; se omitido, usa o nome da struct (ex: "User")
  b.TableName("users")
  // schema da tabela; se omitido, usa o schema selecionado atualmente na conexão
  // (ex: o "search_path" do postgres, ou o schema default do conector)
  b.SchemaName("public")

  // mapea as colunas no banco de dados com os seus tipos explicitos
  b.Col(&t.ID, golem.BIGINT())
  // nome da coluna; se omitido, usa o nome do campo na struct (ex: "Name")
  b.Col(&t.Name, golem.VARCHAR(50)).Name("full_name")
  b.Col(&t.Email, golem.VARCHAR(50))
  b.Col(&t.Age, golem.INT())
  b.Col(&t.CreatedAt, golem.TIMESTAMPTZ())
  b.Col(&t.UpdatedAt, golem.TIMESTAMPTZ())
  b.Col(&t.DeletedAt, golem.TIMESTAMPTZ()).Nullable().Default(nil)

  // aceita diversas propriedades
  b.PrimaryKey(&t.ID)
  // fica fora do Col porque pode ser uma combinação de campos (unique composta)
  b.Unique(&t.Email)

  // declara campos especiais, ou seja campo de criação data/hora e edição data/hora
  b.CreateDate(&t.CreatedAt)
  b.UpdateDate(&t.UpdatedAt)
  b.DeleteDate(&t.DeletedAt).Nullable().Default(nil)
})


var PostEntity = entity.New[Post](func(t *Post, b *entity.Builder) {
  b.Col(&t.ID, golem.BIGINT())
  b.Col(&t.CreatedAt, golem.TIMESTAMPTZ())
  b.Col(&t.UpdatedAt, golem.TIMESTAMPTZ())
  b.Col(&t.DeletedAt, golem.TIMESTAMPTZ()).Nullable().Default(nil)
  b.Col(&t.OwnerUserID, golem.BIGINT())
  b.Col(&t.Title, golem.VARCHAR(50))
  b.Col(&t.Content, golem.TEXT())
  b.Col(&t.Published, golem.BOOLEAN())

  b.PrimaryKey(&t.ID)
  // índice secundário: acelera queries que filtram por OwnerUserID (ex: "posts de um usuário")
  b.Index(&t.OwnerUserID)
  // o terceiro parametro é opcional
  b.ForeignKey(&t.OwnerUserID, UserEntity, relation.NewForeignKeyOptions().
    // em caso de criação de um usuário todos os posts deste usuário serão criados
    // outras opções disponíveis: CascadeInsert, CascadeUpdate, CascadeRemove, CascadeSoftRemove, CascadeRecover
    Cascade(relation.CascadeAll).
    // em caso de deleção de um usuário todos os posts deste usuário serão deletados
    // outras opções são:
    // - relation.OnDeleteRestrict
    // - relation.OnDeleteSetNull
    // - relation.OnDeleteCascade
    // - relation.OnDeleteNoAction
    OnDelete(relation.OnDeleteDefault).
    // em caso de atualização do ID de um usuário todos os posts deste usuário serão atualizados
    // outras opções são:
    // - relation.OnUpdateRestrict
    // - relation.OnUpdateSetNull
    // - relation.OnUpdateCascade
    // - relation.OnUpdateNoAction
    OnUpdate(relation.OnUpdateDefault).
    // indica se constraints de chave estrangeira podem ser adiadas
    // outras opções são:
    // - relation.DeferrableDeferred
    // - relation.DeferrableImmediate
    Deferrable(relation.DeferrableDefault).
    // indica se constraints de chave estrangeira serão criadas para colunas de junção
    // pode ser usado apenas em relações many-to-one e owner one-to-one
    // por padrão é true
    CreateForeignKeyConstraints(true).
    // Define se esta relação será preguiçosa. Note: relações preguiçosas são promessas. Quando você as chama elas
    // retornam uma promessa que resolve o resultado da relação então. Se o tipo da sua propriedade for assíncrona
    // (awaitable) então esta relação é definida como preguiçosa automaticamente.
    Lazy(true).
    // Define se esta relação será ansiosa. Relações ansiosas são sempre carregadas automaticamente quando a entidade
    // proprietária da relação é carregada usando os métodos find*. Só usando o QueryBuilder impede o carregamento de
    // relações ansiosas. A flag eager não pode ser definida de ambos os lados da relação - você pode eager load apenas
    // um lado da relação.
    Eager(true).
    // Indica se a persistência é habilitada para a relação. Por padrão está habilitada, mas se você quiser evitar que
    // qualquer alteração na relação seja refletida no banco de dados, você pode desabilitá-la. Se estiver desabilitada,
    // você só pode alterar uma relação do lado inverso de uma relação ou usando a funcionalidade do construtor de
    // consultas de relação. Isso é útil para otimização de desempenho, pois desabilitar evita múltiplas consultas extras
    // durante o salvamento da entidade.
    Persistence(true).
    // Quando um pai é salvo (com cascata, mas) sem uma linha filha que ainda existe no banco de dados, isso controlará o que
    // deve acontecer com eles. O delete removerá essas linhas do banco de dados. O nullify removerá a chave de relação. O
    // disable manterá a relação intacta. A remoção do item relacionado só é possível através de seu próprio repositório.
    // outras opções são:
    // - relation.OrphanedRowActionNullify
    // - relation.OrphanedRowActionDelete
    // - relation.OrphanedRowActionSoftDelete
    // - relation.OrphanedRowActionDisable
    OrphanedRowAction(relation.OrphanedRowActionNullify)
  )

  // declara campos especiais, ou seja campo de criação data/hora e edição data/hora
  b.CreateDate(&t.CreatedAt)
  b.UpdateDate(&t.UpdatedAt)
  b.DeleteDate(&t.DeletedAt).Nullable().Default(nil)
})

var MessageEntity = entity.New[Message](func(t *Message, b *entity.Builder) {
  b.Col(&t.ID, golem.BIGINT())
  b.Col(&t.CreatedAt, golem.TIMESTAMPTZ())
  b.Col(&t.UpdatedAt, golem.TIMESTAMPTZ())
  b.Col(&t.DeletedAt, golem.TIMESTAMPTZ()).Nullable().Default(nil)
  b.Col(&t.Content, golem.TEXT())
  b.Col(&t.SenderUserID, golem.BIGINT())
  b.Col(&t.PostID, golem.BIGINT())

  b.PrimaryKey(&t.ID)
  b.ForeignKey(&t.SenderUserID, UserEntity)
  b.ForeignKey(&t.PostID, PostEntity)

  b.CreateDate(&t.CreatedAt)
  b.UpdateDate(&t.UpdatedAt)
  b.DeleteDate(&t.DeletedAt).Nullable().Default(nil)
})

// entity.AddHook(Entity) retorna um builder encadeável (mesmo estilo de ForeignKeyOptions/JoinTable):
// cada método fixa um hook slot, sem precisar de tipo wrapper por hook (nada de BeforeCreateHook,
// AfterCreateHook etc). slots disponíveis, todos com a mesma assinatura
// func(ctx context.Context, i *T, conn golem.Conn) error:
//  - (Before|After|OnConflict)Create
//  - (Before|After|OnConflict)Update
//  - (Before|After|OnConflict)Delete
//
// todos os hooks devem retornar um error, e se um error for retornado a operação será cancelada.
// todos rodam dentro da mesma transaction da operação que disparou (conn), então um error de qualquer
// um deles reverte tudo — incluindo o próprio insert/update/delete que disparou o hook.

var _ = entity.AddHook(UserEntity).
  // exemplo de hook antes de criar um usuário:
  BeforeCreate(func (ctx context.Context, i *User, conn golem.Conn) error {
    fmt.Println("antes de criar usuário ", i.Name)
    return nil
  }).
  // exemplo de side-query dentro do hook, na mesma transaction (conn) que criou o usuário:
  AfterCreate(func (ctx context.Context, i *User, conn golem.Conn) error {
    repo := repository.Get(conn, UserEntity)
    count, err := repo.Count(ctx)
    fmt.Println("total de usuários agora:", count)
    return err
  })

/**
 * Nota: se o mesmo hook for chamado 2 vezes então deve disparar um panic informando o slot, Ex:
 * entity.AddHook(UserEntity).
 *   BeforeCreate(func (ctx context.Context, i *User, conn golem.Conn) error {
 *     fmt.Println("antes de criar usuário ", i.Name)
 *     return nil
 *   }).
 *   BeforeCreate(func (ctx context.Context, i *User, conn golem.Conn) error {
 *     fmt.Println("antes de criar usuário ", i.Name)
 *     return nil
 *   })
 */

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity, PostEntity, MessageEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()
}

```

### Many-to-many relations (junction entity)

> Inspirado em https://typeorm.io/docs/relations/relations, mas sem o conceito de `@JoinTable`/`@JoinColumn`:
> a tabela de junção é sempre uma entity comum, com duas foreign keys. Isso é literalmente o que o
> banco de dados faz por baixo dos panos, então não faz sentido esconder isso atrás de uma API paralela
> (`ManyToMany` + `JoinTable`) — `ForeignKey` já resolve o caso.

```go
package ex

import (
  "context"

  golem "github.com/leandroluk/golem"
  entity "github.com/leandroluk/golem/entity"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/adapter/postgres"
)

type Category struct {
  ID   int64
  Name string
}

type Question struct {
  ID    int64
  Title string
  Text  string
}

// tabela de junção: nenhum conceito novo, é uma entity comum com duas foreign keys
type QuestionToCategory struct {
  QuestionID int64
  CategoryID int64
}

var CategoryEntity = entity.New[Category](func(t *Category, b *entity.Builder) {
  b.Col(&t.ID, golem.BIGINT())
  b.Col(&t.Name, golem.VARCHAR(50))

  b.PrimaryKey(&t.ID)
})

var QuestionEntity = entity.New[Question](func(t *Question, b *entity.Builder) {
  b.Col(&t.ID, golem.BIGINT())
  b.Col(&t.Title, golem.VARCHAR(50))
  b.Col(&t.Text, golem.TEXT())

  b.PrimaryKey(&t.ID)
})

// como QuestionToCategory referencia QuestionEntity/CategoryEntity mas nenhuma delas referencia
// QuestionToCategory de volta, não existe ciclo de inicialização — dá pra passar as entities direto
var QuestionToCategoryEntity = entity.New[QuestionToCategory](func(t *QuestionToCategory, b *entity.Builder) {
  b.Col(&t.QuestionID, golem.BIGINT())
  b.Col(&t.CategoryID, golem.BIGINT())

  // chave primária composta
  b.PrimaryKey(&t.QuestionID, &t.CategoryID)

  b.ForeignKey(&t.QuestionID, QuestionEntity)
  b.ForeignKey(&t.CategoryID, CategoryEntity)
})

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(QuestionEntity, CategoryEntity, QuestionToCategoryEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  // sem cascade automático de coleção: cada entity é inserida explicitamente, e a junção é só mais um
  // insert normal — nada de mágica de "salvar o grafo inteiro" por baixo dos panos. tudo dentro de
  // uma única transaction pra não deixar a question "órfã" se a junção falhar
  ctx := context.Background()

  // repository.Get só precisa da conexão (golem.Tx aqui, *golem.DataSource fora de transaction) —
  // ambos implementam golem.Conn
  err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
    categories, err := repository.Get(tx, CategoryEntity).InsertMany(ctx,
      &Category{Name: "ORMs"},
      &Category{Name: "Programming"},
    )
    if err != nil {
      return err
    }

    question, err := repository.Get(tx, QuestionEntity).Insert(ctx, &Question{
      Title: "Como perguntar sobre o golem?",
      Text:  "Onde posso tirar dúvidas relacionadas ao golem?",
    })
    if err != nil {
      return err
    }

    _, err = repository.Get(tx, QuestionToCategoryEntity).InsertMany(ctx,
      &QuestionToCategory{QuestionID: question.ID, CategoryID: categories[0].ID},
      &QuestionToCategory{QuestionID: question.ID, CategoryID: categories[1].ID},
    )
    return err
  })
  if err != nil {
    panic(err)
  }
}
```

> **Nota**: pra carregar `question.Categories` como coleção pronta (tipo eager/lazy loading do TypeORM),
> a ideia é resolver isso depois via helper de query (`Preload`/`With` sobre `QuestionToCategoryEntity`),
> não como um novo tipo de relação. Mantém a API de relações só com `ForeignKey` (one-to-many/many-to-one/
> one-to-one), que é o que existe de fato no banco.

### Repository (CRUD)

> Sem `Manager` genérico: só existe `Repository[T]`, sempre amarrado a uma entity só (equivalente ao
> `dataSource.getRepository(User)` do TypeORM, sem o `dataSource.manager` paralelo). Transaction vira
> responsabilidade do `DataSource`.
>
> `repository.Get[T any](conn golem.Conn, e *entity.Entity[T]) *Repository[T]` recebe uma `golem.Conn` —
> interface implementada tanto por `*golem.DataSource` quanto por `golem.Tx`. Sem transaction, passa o
> `dataSource`; dentro de uma, passa a `tx` — o repository nem sabe (nem precisa saber) qual dos dois é.

```go
package ex

import (
  "context"
  "fmt"

  golem "github.com/leandroluk/golem"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/adapter/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity, PostEntity, MessageEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()

  // repository.Get[T] infere T pelo tipo de UserEntity (*entity.Entity[User]), não precisa escrever
  // repository.Get[User](...). dataSource implementa golem.Conn, então roda no pool dele
  users := repository.Get(dataSource, UserEntity)

  // Insert: sempre 1 entity nova, retorna com a PK preenchida
  user, err := users.Insert(ctx, &User{Name: "Leandro", Email: "leandroluk@gmail.com", Age: 30})
  if err != nil {
    panic(err)
  }

  // SaveOne: já tenho a instância em runtime (veio do Insert acima) — persiste de novo por PK
  user.Age = 31
  user, err = users.SaveOne(ctx, &user)
  if err != nil {
    panic(err)
  }

  // busca por chave primária
  found, err := users.FindByID(ctx, user.ID)
  if err != nil {
    panic(err)
  }

  // Find/FindOne recebem critérios — detalhado na seção "Query Builder" mais abaixo
  admins, err := users.FindMany(ctx /*, critérios aqui */)
  if err != nil {
    panic(err)
  }

  // Count/Exists recebem um critério próprio (query.Count[T], só Where) — detalhado na seção "Query
  // Builder" mais abaixo; sem argumento nenhum, contam/checam a tabela toda
  total, err := users.Count(ctx)
  if err != nil {
    panic(err)
  }
  fmt.Println("total de usuários:", total)

  // User tem DeleteDate declarado, então isso é soft delete (seta DeletedAt), não apaga a linha
  if err := users.Delete(ctx, &found); err != nil {
    panic(err)
  }

  // Restore desfaz: limpa DeletedAt de novo
  if err := users.Restore(ctx, &found); err != nil {
    panic(err)
  }

  // Transaction fica no DataSource, não no Repository. dentro do callback, tx (que também implementa
  // golem.Conn) substitui o dataSource na hora de montar o repository.Get
  err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
    if _, err := repository.Get(tx, PostEntity).Insert(ctx, &Post{OwnerUserID: user.ID, Title: "primeiro post"}); err != nil {
      return err
    }
    _, err := repository.Get(tx, MessageEntity).Insert(ctx, &Message{SenderUserID: user.ID, Content: "primeira mensagem"})
    return err
  })
  if err != nil {
    panic(err)
  }

  _ = admins
}
```

> **Referência rápida** — métodos de `Repository[T]`:
>
> | Método | Retorno | Descrição |
> | :- | :- | :- |
> | `Insert(ctx, e *T) (T, error)` | 1 registro | insere 1 entity nova, retorna com a PK preenchida |
> | `InsertMany(ctx, entities ...*T) ([]T, error)` | N registros | insere várias de uma vez |
> | `SaveOne(ctx, e *T) (T, error)` | 1 registro | persiste de novo uma instância que você já tem em runtime (ex: veio de um `Insert`/`FindByID` anterior), por PK |
> | `SaveMany(ctx, entities ...*T) ([]T, error)` | N registros | igual `SaveOne`, pra várias instâncias de uma vez |
> | `UpdateOne(ctx, criteria func(t *T, u *query.Update[T])) (T, error)` | 1 registro | atualiza direto no banco por critério (`Where`+`Set`), sem precisar ter a instância em runtime |
> | `UpdateMany(ctx, criteria func(t *T, u *query.Update[T])) ([]T, error)` | N registros | igual `UpdateOne`, mas o critério pode casar mais de uma linha |
> | `Delete(ctx, entities ...*T) error` | — | delete por PK; se a entity tem `DeleteDate`, seta o timestamp (soft delete) em vez de apagar a linha |
> | `Restore(ctx, entities ...*T) error` | — | desfaz soft delete (limpa `DeleteDate`) por PK; sem `DeleteDate` declarado, é no-op |
> | `FindByID(ctx, id any) (T, error)` | 1 registro | busca por PK |
> | `FindMany(ctx, criteria ...func(t *T, q *query.Query[T])) ([]T, error)` | N registros | critério opcional; sem ele, traz a tabela toda. detalhado na seção Query Builder |
> | `FindOne(ctx, criteria ...func(t *T, q *query.Query[T])) (T, error)` | 1 registro | igual `FindMany`, limita 1 |
> | `Count(ctx, criteria ...func(t *T, c *query.Count[T])) (int64, error)` | contagem | critério opcional (só `Where`); sem ele, conta a tabela toda |
> | `Exists(ctx, criteria ...func(t *T, c *query.Count[T])) (bool, error)` | bool | atalho pra `Count > 0` sem trazer linha, mesmo critério de `Count` |
>
> `dataSource.Transaction(ctx, func(tx golem.Tx) error {...})` abre a transaction; qualquer
> `repository.Get(tx, Entity)` criado dentro do callback roda nela (`tx` implementa `golem.Conn`, igual
> `dataSource`). Se o callback retornar error, a transaction é revertida por inteiro.

### Query Builder

> Critérios via closure declarativa: o callback recebe `t *T` (mesmo field pointer de `Col`/`ForeignKey`/
> `PrimaryKey`) e `q *query.Query[T]`. Condições em `Where` são variádicas com semântica AND. Ordem das
> chamadas dentro do callback não importa — a query só é montada quando o callback termina.
>
> Se a entity tem `DeleteDate` (soft delete), toda consulta filtra os deletados por padrão (`WHERE
> deleted_at IS NULL` implícito) — igual todo ORM com soft delete faz. `.WithDeleted()` desliga esse
> filtro pra essa consulta. Existe em qualquer builder que tenha `Where` por baixo: `query.Query[T]`
> (`FindMany`/`FindOne`), `query.Count[T]` (`Count`/`Exists`), `query.Update[T]` (`UpdateOne`/`UpdateMany`)
> e `query.Join[T]` (dentro de `join.*`). Em entities sem `DeleteDate`, `.WithDeleted()` é um no-op.

```go
package ex

import (
  "context"
  "fmt"

  golem "github.com/leandroluk/golem"
  op "github.com/leandroluk/golem/op"
  query "github.com/leandroluk/golem/query"
  postgres "github.com/leandroluk/golem/adapter/postgres"
  repository "github.com/leandroluk/golem/repository"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity, PostEntity, MessageEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()

  // repository.Get[T] infere T pelo tipo de UserEntity (*entity.Entity[User]), não precisa escrever
  // repository.Get[User](...). dataSource implementa golem.Conn, então roda no pool dele
  userRepo := repository.Get(dataSource, UserEntity)

  // Insert: sempre 1 entity nova, retorna com a PK preenchida
  user, err := userRepo.Insert(ctx, &User{Name: "Leandro", Email: "leandroluk@gmail.com", Age: 30})
  if err != nil {
    panic(err)
  }

  // SaveOne: já tenho a instância em runtime (veio do Insert acima) — persiste de novo por PK
  user.Age = 31
  user, err = userRepo.SaveOne(ctx, &user)
  if err != nil {
    panic(err)
  }

  // SaveMany: mesma ideia, só que pra várias instâncias já em runtime (variádico, igual InsertMany)
  users, err := userRepo.SaveMany(ctx, &user)
  if err != nil {
    panic(err)
  }
  _ = users

  // UpdateOne: sem instância nenhuma — só Where+Set, atualiza direto no banco
  updated, err := userRepo.UpdateOne(ctx, func(t *User, u *query.Update[User]) {
    u.Where(op.Eq(&t.ID, user.ID))
    u.Set(&t.Name, "Leandro")
    u.Set(&t.Email, "leandroluk@gmail.com")
    u.Set(&t.Age, 30)
  })
  if err != nil {
    panic(err)
  }
  _ = updated

  // UpdateMany: mesma ideia, mas o critério pode casar (e atualizar) mais de uma linha
  users, err = userRepo.UpdateMany(ctx, func(t *User, u *query.Update[User]) {
    u.Where(op.Eq(&t.Age, 30))
    u.Set(&t.Age, 31)
  })
  if err != nil {
    panic(err)
  }
  _ = users

  // busca por chave primária
  found, err := userRepo.FindByID(ctx, user.ID)
  if err != nil {
    panic(err)
  }

  admin, err := userRepo.FindOne(ctx, func (t *User, q *query.Query[User]) {
    q.Select(&t.Name, &t.Email, &t.Age)
    q.Where(
      // aqui tendo outras condições como op.Or, op.In, op.Like, etc — op.Not(op.In(...)) compõe negação, sem NotIn dedicado
      op.Eq(&t.Name, "Leandro"),
      op.Eq(&t.Email, "leandroluk@gmail.com"),
      op.Eq(&t.Age, 30),
    )
    q.OrderBy(op.Desc(&t.ID))
    q.Limit(10)
    q.Offset(0)
    // sem isso, usuários com DeletedAt preenchido não apareceriam nesse FindOne
    q.WithDeleted()
  })
  if err != nil {
    panic(err)
  }

  admins, err := userRepo.FindMany(ctx, func (t *User, q *query.Query[User]) {
    // se não for passado o q.Select sempre traz todas as colunas
    q.Where(
      // aqui tendo outras condições como op.Or, op.In, op.Like, etc — op.Not(op.In(...)) compõe negação, sem NotIn dedicado
      op.Eq(&t.Name, "Leandro"),
      op.Eq(&t.Email, "leandroluk@gmail.com"),
      op.Eq(&t.Age, 30),
    )
    // a ordem das declarações não importam, a query é construída quando a função se encerra
    q.Limit(10)
    q.Offset(0)
    q.OrderBy(op.Desc(&t.ID))
  })
  if err != nil {
    panic(err)
  }
  _ = admins

  // Count/Exists recebem um critério próprio (query.Count[T], só Where); sem argumento nenhum,
  // contam/checam a tabela toda
  adultCount, err := userRepo.Count(ctx, func (t *User, c *query.Count[User]) {
    c.Where(op.Gte(&t.Age, 18))
  })
  if err != nil {
    panic(err)
  }
  fmt.Println("total de usuários adultos:", adultCount)

  if err := userRepo.Delete(ctx, &found); err != nil {
    panic(err)
  }

  // Transaction fica no DataSource, não no Repository. dentro do callback, tx (que também implementa
  // golem.Conn) substitui o dataSource na hora de montar o repository.Get
  err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
    if _, err := repository.Get(tx, PostEntity).Insert(ctx, &Post{OwnerUserID: user.ID, Title: "primeiro post"}); err != nil {
      return err
    }
    _, err := repository.Get(tx, MessageEntity).Insert(ctx, &Message{SenderUserID: user.ID, Content: "primeira mensagem"})
    return err
  })
  if err != nil {
    panic(err)
  }
}
```

### Joins

> Pacote `golem/join`: `join.Inner`/`join.Left`/`join.Right`/`join.Full` (nomes de `JOIN` do SQL — `Left`/
> `Right`/`Full` já são "outer" por definição, não precisa de um `Outer` genérico separado). Cada um
> recebe a `*query.Query[T]` de fora (pra se registrar nela), a entity do lado que está entrando no join
> (`T` inferido dela, igual `repository.Get`) e um callback com o lado novo. Nomeamos os builders `q0`
> (de fora), `q1` (do join) etc pra deixar explícito o nível de cada um.
>
> `q1.On(fieldPtr, fieldPtr)` compara coluna com coluna (os dois lados são endereço de campo) — diferente
> de `op.Eq(fieldPtr, valor)`, que compara coluna com valor literal no `Where`. Por isso é um método
> separado em vez de reaproveitar `op.Eq` pros dois casos. `q1.Where(...)` também existe, pra filtrar
> pelo lado que entrou no join (com `op.*` normal, valor literal) sem misturar com o `Where` da query de fora.

```go
package ex

import (
  "context"

  golem "github.com/leandroluk/golem"
  op "github.com/leandroluk/golem/op"
  join "github.com/leandroluk/golem/join"
  query "github.com/leandroluk/golem/query"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/adapter/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity, PostEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()

  // usuários que têm pelo menos 1 post publicado (INNER: só entra quem casa a condição do On)
  users, err := repository.Get(dataSource, UserEntity).FindMany(ctx, func (u *User, q0 *query.Query[User]) {
    join.Inner(q0, PostEntity, func (p *Post, q1 *query.Join[Post]) {
      q1.On(&p.OwnerUserID, &u.ID)
      q1.Where(op.Eq(&p.Published, true))
    })
    q0.Where(op.Eq(&u.Name, "Leandro"))
  })
  if err != nil {
    panic(err)
  }
  _ = users
}
```

### Raw SQL (escape hatch)

> Nenhum builder cobre 100% dos casos, então precisa de uma saída pra SQL cru em dois níveis:
>
> `Exec` faz parte de `golem.Conn` (então funciona igual em `*golem.DataSource` e em `golem.Tx` dentro de uma
> transaction) — roda qualquer statement e devolve um `golem.Result`, porque consultas diferentes devolvem
> referências diferentes (uma `SELECT` tem linhas pra iterar, um `UPDATE` sem `RETURNING` só tem contagem
> afetada, um `UPDATE ... RETURNING` tem os dois). `golem.Result`:
>
>  type Result interface {
>    // avança pro próximo registro (igual sql.Rows.Next); false quando acabam as linhas ou não há nenhuma
>    Next() bool
>    // linha atual como coluna→valor; só válido depois de Next() == true
>    Scan() (map[string]any, error)
>    // linhas afetadas pelo statement; não depende de Next/Scan, funciona mesmo sem RETURNING
>    RowsAffected() (int64, error)
>  }
>
> `Repository[T].Exec` roda uma SQL de leitura crua mas escaneia o resultado pro tipo `T` (retorna `[]T`
> direto, sem precisar de `Result`), reaproveitando o mapeamento coluna→campo que já existe (o mesmo nome
> declarado em `Col(...).Name(...)`) — sem precisar de tags nem scanning manual.

```go
package ex

import (
  "context"
  "fmt"

  golem "github.com/leandroluk/golem"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/adapter/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()

  // Exec: devolve golem.Result (golem.Conn: dataSource ou tx, tanto faz). RowsAffected funciona mesmo sem
  // RETURNING; Next/Scan só trazem linha se o statement devolver alguma (aqui, via RETURNING)
  result, err := dataSource.Exec(ctx, "UPDATE users SET age = age + 1 WHERE id = $1 RETURNING *", 1)
  if err != nil {
    panic(err)
  }
  affected, err := result.RowsAffected()
  if err != nil {
    panic(err)
  }
  fmt.Println("linhas afetadas:", affected)

  for result.Next() {
    row, err := result.Scan()
    if err != nil {
      panic(err)
    }
    fmt.Println(row)
  }

  // Exec no Repository[T]: mesma ideia, mas escaneia o resultado pro tipo da entity
  users, err := repository.Get(dataSource, UserEntity).Exec(ctx, "SELECT * FROM users WHERE age > $1", 18)
  if err != nil {
    panic(err)
  }
  _ = users
}
```

> **Referência rápida** adicional:
>
> | Método | Onde | Retorno | Descrição |
> | :- | :- | :- | :- |
> | `Exec(ctx, sql string, args ...any) (golem.Result, error)` | `golem.Conn` (`DataSource`/`Tx`) | `Result` (`Next`/`Scan`/`RowsAffected`) | statement cru, sem tipo amarrado |
> | `Exec(ctx, sql string, args ...any) ([]T, error)` | `Repository[T]` | N registros | leitura crua, escaneada pro tipo `T` |

### Errors

> Sentinels em `golem` (`golem.Err*`) pros casos mais comuns — sempre checa com `errors.Is`, nunca compara
> mensagem de string (cada dialeto/driver fala diferente). O adapter (`postgres`, etc.) é quem traduz o
> erro nativo do driver (ex: SQLSTATE `23505` do postgres) pro sentinel correspondente, preservando o
> erro original por baixo via `%w` — então `errors.Unwrap`/`errors.As` ainda alcança o erro nativo do
> driver quando o sentinel não é granular o suficiente.
>
> conjunto inicial (mais comuns; a lista cresce conforme mais dialetos/casos forem mapeados, sem quebrar
> o que já existe):
>
>  - `golem.ErrNotFound`: nenhum registro encontrado (`FindOne`/`FindByID`/`UpdateOne` sem match)
>  - `golem.ErrDuplicateKey`: violação de `Unique` (coluna ou composta)
>  - `golem.ErrForeignKeyViolation`: violação de `ForeignKey` (aponta pra algo que não existe, ou deleta algo ainda referenciado)
>
> se o driver devolver um erro que não bate com nenhum sentinel mapeado ainda, o erro original sobe sem
> alteração (sem forçar num sentinel genérico tipo "unknown") — só vira sentinel o que já foi mapeado.

```go
package ex

import (
  "context"
  "errors"
  "fmt"

  golem "github.com/leandroluk/golem"
  repository "github.com/leandroluk/golem/repository"
)

func handle(ctx context.Context, userRepo *repository.Repository[User]) {
  user, err := userRepo.FindByID(ctx, 999)
  if errors.Is(err, golem.ErrNotFound) {
    fmt.Println("usuário não existe")
    return
  }
  if err != nil {
    panic(err) // erro de infra de verdade (conexão caiu, etc.)
  }

  _, err = userRepo.Insert(ctx, &User{Email: user.Email})
  if errors.Is(err, golem.ErrDuplicateKey) {
    fmt.Println("email já cadastrado")
    return
  }
  if err != nil {
    panic(err)
  }
}
```

### Migrations

> Fora de escopo, de propósito. Entities aqui descrevem mapeamento/comportamento em runtime, não são
> fonte de verdade de DDL — schema (criação/alteração de tabelas, versionamento, rollback) fica pra
> ferramenta externa (Liquibase, Flyway, goose, etc.). Sem "synchronize" automático tipo TypeORM dev mode.

### Custom logger

```go
package ex

import (
  golem "github.com/leandroluk/golem"
  postgres "github.com/leandroluk/golem/adapter/postgres"
)

type Logger struct {}

// garante que a struct Logger implementa a interface golem.Logger
var _ golem.Logger = (*Logger)(nil)

func (l *Logger) Log(level golem.LogLevel, msg string, args map[string]any) {
  raw, _ := json.Marshal(args)
  str := string(raw)
  switch level {
  case golem.LogLevelDebug: fmt.Println("[DEBUG]: " + msg, str)
  case golem.LogLevelInfo: fmt.Println("[INFO]: " + msg, str)
  case golem.LogLevelWarn: fmt.Println("[WARN]: " + msg, str)
  case golem.LogLevelError: fmt.Println("[ERROR]: " + msg, str)
  default: panic("log level desconhecido " + level.String())
  }
}
func (l *Logger) Debug(msg string, args map[string]any) { l.Log(golem.LogLevelDebug, msg, args) }
func (l *Logger) Info(msg string, args map[string]any)  { l.Log(golem.LogLevelInfo,  msg, args) }
func (l *Logger) Warn(msg string, args map[string]any)  { l.Log(golem.LogLevelWarn,  msg, args) }
func (l *Logger) Error(msg string, args map[string]any) { l.Log(golem.LogLevelError, msg, args) }

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      // define qual logger será usado, por padrão usa uma implementação com fmt.Println
      o.Logger = &Logger{}
    })
  )
}
```

---

## Contributors
Thanks to all the people who contribute! [[Contribute](CONTRIBUTING.md)]

---

## License
MIT License – see [LICENSE](LICENSE) file for details.
