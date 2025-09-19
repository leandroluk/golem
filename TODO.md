# To do

1.  Query Builder fluente [OK]()

    - Exemplo (TypeORM / Prisma / SQLAlchemy):

      ```go
      users, _ := userModel.
        Where(Where(userSchema, func(u *User) *string { return &u.Email }).Like("%gmail.com")).
        OrderBy("created_at", -1).
        Limit(10).
        FindMany(ctx)
      ```

    - Hoje você já tem core.Query, mas poderia criar uma API fluente em cima, que constrói a query com encadeamento.

    - Benefício: menos boilerplate, mais legibilidade.

2.  Relations (Eager & Lazy Loading) [OK]()

    - Exemplo (TypeORM / Hibernate / Mongoose Populate):

      ```go
      order, _ := orderModel.FindOne(ctx, query, WithRelation("user"))
      ```

    - Tipos de relação:

      - OneToOne, OneToMany, ManyToMany

    - Benefício: facilita trabalhar com entidades conectadas sem fazer manualmente joins ou várias queries.

3.  Migrations & Schema Sync [OK (usando golang migrate)]()

    - Exemplo (Prisma migrate / TypeORM migration / Alembic SQLAlchemy):
  
      - CLI que gera SQL de criação/alteração com base nas entidades.

      - `golem migrate dev`, `golem migrate deploy`.

    - Benefício: evolução controlada do schema, versionamento e CI/CD integrado.

4.  Soft Deletes & Timestamps [Ok]()

    - Exemplo (Mongoose { timestamps: true }, TypeORM @DeleteDateColumn)

    - Adicionar suporte automático para:

      - createdAt, updatedAt (auto-fill)

      - deletedAt para soft delete (Delete → update com timestamp em vez de remover).

    - Benefício: padrão em apps reais.

5.  Validation e Constraints no Schema [Não será feito]()

    - Exemplo (Mongoose validators / Hibernate annotations / Prisma constraints)

    - Hoje já tem IsRequired, IsUnique. Poderia evoluir para:

      - Validators customizáveis (MinLength, Regex, Range).

      - Execução automática no PreInsert/PreUpdate.

    - Benefício: segurança e integridade mais próximas da camada de domínio.

6.  Aggregations & Raw Queries [Não será feito]()

    - Exemplo (Mongoose Aggregation, Prisma queryRaw, SQLAlchemy session.execute)

      ```go
      userModel.Aggregate(ctx, pipeline)
      userModel.Raw(ctx, "SELECT COUNT(*) FROM users WHERE email LIKE $1", "%gmail.com")
      ```

    - Benefício: poder quando precisar sair da camada ORM sem perder o driver.

7.  Transactions mais ergonômicas [Ok]()

    - Exemplo (Prisma `$transaction`, TypeORM `manager.transaction`)
    
      ```go
      err := db.Transaction(ctx, func(txCtx context.Context) error {
        if err := userModel.Create(txCtx, &user); err != nil {
          return err
        }
        return orderModel.Create(txCtx, &order)
      })
      ```

    - Benefício: abstrai Begin/Commit/Rollback em função de callback.

8.  Change Tracking & Unit of Work [Não será feito]()

    - Exemplo (Hibernate Session, SQLAlchemy session tracking)

    - O ORM poderia manter um contexto de entidades carregadas e saber o que mudou (dirty checking).

    - No commit, gera automaticamente os updates.

    - Benefício: simplifica muito o uso, mas aumenta complexidade.

9.  Hooks Globais & Middleware [Ok]()

    - Além de hooks por entidade, permitir middlewares globais:

      ```go
      core.Use(func(next core.Handler) core.Handler {
        return func(ctx context.Context, op core.Operation, payload any) error {
          log.Println("executando", op)
          return next(ctx, op, payload)
        }
      })
      ```

    - Benefício: logging, tracing, auditoria em todas operações sem repetir código.

10. Code Generation & CLI [Não será feito]()

    - Exemplo (Prisma generate / TypeORM CLI / SQLAlchemy alembic)

    - Geração de schemas e modelos a partir do banco (reverse engineering).

    - Geração de boilerplate (golem generate model User).

    - Benefício: acelera muito onboarding.

11. Multi-tenant / Sharding [Ok]()

    - Já falou em multi-tenant no início. ORM pode dar suporte oficial:

      ```go
      userModel.WithTenant("tenant_a").FindMany(ctx, nil)
      ```

    - Benefício: evita reinventar a roda quando precisar separar DBs ou schemas.

12. Advanced Features

    - Caching integrado (Redis + hooks de invalidar). [Ok]()

    - Optimistic Locking (version field, igual Hibernate). [Não será feito]()

    - Query Debug Logging (igual Prisma `DEBUG=*` ou TypeORM `logging: true`). [Ok]()

    - Schema Diff visual (como Prisma Studio). [Não será feito]()