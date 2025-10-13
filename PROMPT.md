estou querendo criar um orm para golang que permita eu construir um schema recebendo uma struct como referência e de forma agnóstica. espero que minha lib permita a construção de algo assim:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"
	g "github.com/leandroluk/golem/core"
	gp "github.com/leandroluk/golem/driver/postgres"
)

// utilities

func Ptr[T any](v T) *T { return &v }

// ~=~=~= types ~=~=~=

type UserRole string

var (
	UserRoleAdmin UserRole = "ADMIN"
	UserRoleUser  UserRole = "USER"
)

type User struct {
	ID         int64      `db:"id"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
	DeletedAt  *time.Time `db:"updated_at"`
	FirstName  string     `db:"first_name"`
	LastName   string     `db:"last_name"`
	Email      string     `db:"email"`
	Password   string     `db:"password"`
	IsActive   bool       `db:"is_active"`
	OTP        string     `db:"otp"`
	PictureURL *string    `db:"picture_url"`
	Role       UserRole   `db:"role"`
}

type Post struct {
	ID        int64     `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Content   string    `db:"content"`
	UserID    int64     `db:"user_id"`
}

// ~=~=~= schemas ~=~=~=

var encrypt g.GetFN[string] = func(v string) (out string) {
	out = v
}

var decrypt g.SetFN[string] = func(v string) (out string) {
	out = v
}

var UserSchema = g.NewSchema[User](func(u *User, s *g.Schema, o g.Options) *g.Schema {
	return s.
		PrimaryField(&u.ID, gp.BigInt()).
		CreateDateField(&u.CreatedAt, gp.Timestamptz(3)).               // quando cria define seta o valor
		UpdateDateField(&u.CreatedAt, gp.Timestamptz(3)).               // quando cria e edita define seta o valor
		DeleteDateField(&u.DeletedAt, gp.Timestamptz(3), o.Nullable()). // indica que é deleção virtual
		// qualquer outro campo comum com valores default
		Field(&u.FirstName, gp.VarChar(100), o.Default("")).
		Field(&u.LastName, gp.VarChar(100), o.Default("")).
		// define que é um campo
		Field(&u.Email, gp.VarChar(100), o.Default(""), o.Unique()).
		// modificar os get e set, usando para fazer parsing e outras coisas na estrutura
		Field(&u.Password, gp.Text(), o.Getter(encrypt), o.Setter(decrypt)).
		Field(&u.IsActive, gp.Boolean(), o.Default(false)).
		Field(&u.OTP, gp.Char(6)).
		Field(&u.PictureURL, gp.Text(), o.Nullable()).
		Field(&u.Role, gp.VarChar(10), o.Enum(UserRoleAdmin, UserRoleUser), o.Default(UserRoleUser))
})
var PostSchema = g.NewSchema[Post](func(p *Post, s *g.Schema, o g.Options) *g.Schema {
	return s.
		PrimaryField(&p.ID, gp.BigInt()).
		CreateDateField(&p.CreatedAt, gp.Timestamptz(3)).
		UpdateDateField(&p.CreatedAt, gp.Timestamptz(3)).
		Field(&p.Content, gp.Text()).
		// precisa resolver sozinho com base na struct quando for consumir o schema
		ForeignField(&p.UserID, o.Reference[User]())
})

// ~=~=~= migration's ~=~=~=

var CreateExtensions001 = g.NewMigration("001"). // o value é a tag na ordem da migration
	Up(
		`CREATE EXTENSION IF NOT EXISTS "fuzzystrmatch"`,
		`CREATE EXTENSION IF NOT EXISTS "pg_trgm"`).
	Down(
		`DROP EXTENSION IF NOT EXISTS fuzzystrmatch;`,
		`DROP EXTENSION IF NOT EXISTS pg_trgm;`)

var CreateTableUser002 = g.NewMigration("002").
	Up(`CREATE TABLE "user" (/* script de criação da tabela de usuário aqui */)`).
	Down(`DROP TABLE "user"`)

var CreateTablePost003 = g.NewMigration("002").
	Up(`CREATE TABLE "post" (/* script de criação da tabela de post aqui */)`).
	Down(`DROP TABLE "post"`)

// ~=~=~= main ~=~=~=

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := g.NewClient().
		// pra registrar os schemas e gerar as referências como de primary/foreign key's e unique's
		// definido fora da ordem para não fazer diferença entre as relações
		Schemas(PostSchema, UserSchema).
		// deve disparar erro caso já exista alguma migration com a mesma tag
		// importado fora da ordem de propósito pra deduzir que não faz diferença, a tag que define a ordem
		Migrations(CreateExtensions001, CreateTablePost003, CreateTableUser002).
		// conecta no banco passando o contexto atual e o cancel pra cancelar caso de erro
		Connect(ctx)

	if err := client.Ping(); err != nil {
		log.Fatal(err)
	}

	userModel, _ := client.Model(ctx, UserSchema)
	postModel, _ := client.Model(ctx, PostSchema)

	// inserindo itens retornando as instâncias inseridas

	john := userModel.Save(ctx, &User{
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john.doe@email.com",
		Password:   "Test@123",
		OTP:        "123456",
		PictureURL: Ptr("https://placehold.co/192x192/FF0000/FFFFFF/png"),
		Role:       UserRoleAdmin,
	})

	// inserindo pela instancia da struct

	postModel.Insert(
		&Post{Content: "a", UserID: john.ID},
		&Post{Content: "b", UserID: john.ID},
		&Post{Content: "c", UserID: john.ID})

	// editando valores por condição

	postModel.Update(ctx, func(p *Post, u *g.Update, q *g.Query) {
		u.Set(&p.Content, "new a")
		q.Eq(&p.UserID, john.ID) // e qualquer outro operador necessário nesse formato
	})

	// deletando valores por condição

	postModel.Delete[Post](ctx, func(p *Post, q *g.Query) { q.Like(&p.Content, `a`) })

	// contado valores

	count, err := postModel.Count(ctx, func(p *Post, q *g.Query) { q.Eq(&p.UserID, john.ID) })
	if err != nil {
		log.Fatal(err)
	}

	log.Println(fmt.Sprintf("total de posts: %d", count))

	// selecionando dados do banco

	// retorna *User
	user, err := userModel.FindOne(ctx, func(u *User, w *g.Query) { w.Eq(&u.ID, 1) })
	if err != nil {
		log.Fatal(err)
	}

	// retorna []Post
	postsFromUser, err := postModel.FindMany(ctx, func(p *Post, q *g.Query) {
		q.Eq(&p.UserID, &user.ID)
		q.Skip(10)
		q.Limit(100)
	})

	// auto cast para json de uma consulta retornando json.RawMessage
	userJSON, err := userModel.FindOne(ctx, func(u *User, w *g.Query) { w.Eq(&u.ID, 1) }, g.AsJson())
	postsFromUserJSON, err := postModel.FindMany(func(p *Post, q *g.Query) {
		q.Eq(&p.UserID, &user.ID)
		q.Skip(10)
		q.Limit(100)
	}, g.AsJson())

	// execução de queries do tipo raw via client sem usar model

	clientRaw, err := client.Query(ctx, `SELECT 1`)                  // retorna [][]any
	clientJSON, err := client.Query(ctx, `SELECT 1`, g.AsJson()) // retorna []json.RawMessage

	// retorna []User mapeando os campos que encontrar com base nos campos mapeados no json
	clientUser, err := client.Query(ctx, `SELECT * from "user"`, g.AsType[User]())

	// fechando conexão
	client.Close()
}

```