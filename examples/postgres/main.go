package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/leandroluk/golem/builder"
	"github.com/leandroluk/golem/core"
	"github.com/leandroluk/golem/driver/postgres"
)

//#region utils

func Ptr[T any](v T) *T { return &v }

//#endregion

// ~=~=~= domain types ~=~=~=

type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

type User struct {
	ID         int64      `json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	Email      string     `json:"email"`
	Password   string     `json:"password"`
	IsActive   bool       `json:"is_active"`
	OTP        string     `json:"otp"`
	PictureURL *string    `json:"picture_url"`
	Role       UserRole   `json:"role"`
}

type Post struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Content   string    `json:"content"`
	UserID    int64     `json:"user_id"`
}

// ~=~=~= schemas ~=~=~=

var UserSchema = core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
	return s.WithTable("user").
		Field(&u.ID, postgres.BigInt(), core.Primary()).
		Field(&u.CreatedAt, postgres.Timestamptz(3), core.CreatedAt()).
		Field(&u.UpdatedAt, postgres.Timestamptz(3), core.UpdatedAt()).
		Field(&u.DeletedAt, postgres.Timestamptz(3), core.Nullable(), core.DeletedAt()).
		Field(&u.FirstName, postgres.VarChar(100), core.Default("")).
		Field(&u.LastName, postgres.VarChar(100), core.Default("")).
		Field(&u.Email, postgres.VarChar(100), core.Unique()).
		Field(&u.Password, postgres.Text()).
		Field(&u.IsActive, postgres.Boolean(), core.Default(false)).
		Field(&u.OTP, postgres.Char(6)).
		Field(&u.PictureURL, postgres.Text(), core.Nullable()).
		Field(&u.Role, postgres.VarChar(10), core.Enum(UserRoleAdmin, UserRoleUser), core.Default(UserRoleUser))
})

var PostSchema = core.NewSchema(func(p *Post, s *core.Schema) *core.Schema {
	return s.WithTable("post").
		Field(&p.ID, postgres.BigInt(), core.Primary()).
		Field(&p.CreatedAt, postgres.Timestamptz(3), core.CreatedAt()).
		Field(&p.UpdatedAt, postgres.Timestamptz(3), core.UpdatedAt()).
		Field(&p.Content, postgres.Text()).
		Field(&p.UserID, core.Reference[User]())
})

// ~=~=~= migrations ~=~=~=

var sqls = []string{
	`DROP TABLE IF EXISTS "post";`,
	`DROP TABLE IF EXISTS "user";`,
	`CREATE TABLE "user" (
		"id"          BIGSERIAL      NOT NULL,
		"created_at"  TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		"updated_at"  TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		"deleted_at"  TIMESTAMPTZ(3)     NULL DEFAULT CURRENT_TIMESTAMP(3),
		"first_name"  VARCHAR(100)   NOT NULL DEFAULT '',
		"last_name"   VARCHAR(100)   NOT NULL DEFAULT '',
		"email"       VARCHAR(100)   NOT NULL DEFAULT '',
		"password"    TEXT           NOT NULL,
		"is_active"   BOOLEAN        NOT NULL DEFAULT false,
		"otp"         CHAR(6)        NOT NULL,
		"picture_url" TEXT               NULL,
		"role"        VARCHAR(10)    NOT NULL DEFAULT 'user',
		--
		PRIMARY KEY ("id"),
		UNIQUE      ("email")
  );`,
	`CREATE TABLE "post" (
		"id"         BIGSERIAL      NOT NULL,
		"created_at" TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		"updated_at" TIMESTAMPTZ(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		"content"    TEXT               NULL,
		"user_id"    BIGINT         NOT NULL,
		--
		PRIMARY KEY ("id"),
		FOREIGN KEY ("user_id") REFERENCES "user" ("id")
	);`,
}

func migrate(ctx context.Context, client *core.Client[string]) {
	for _, sql := range sqls {
		if _, err := client.Exec(ctx, sql); err != nil {
			log.Fatal(err)
		}
	}
}

// ~=~=~= main ~=~=~=

func main() {
	ctx := context.Background()

	driver := postgres.NewDriver("postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	client := core.NewClient(driver).Schemas(UserSchema, PostSchema)

	if err := client.Connect(ctx); err != nil {
		log.Fatal("erro connect:", err)
	}
	defer client.Close()

	migrate(ctx, client)

	// cria models
	userModel := builder.NewModel[User](UserSchema, client)
	postModel := builder.NewModel[Post](PostSchema, client)

	// ========== CREATE ==========
	ts := time.Now().UnixMilli()
	john := &User{
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john@example.com",
		Password:   "secret",
		OTP:        fmt.Sprintf("%06d", ts%1_000_000),
		PictureURL: Ptr("https://placehold.co/100x100"),
		Role:       UserRoleAdmin,
	}
	userModel.Save(ctx, john)

	j, _ := json.Marshal(john)
	fmt.Println("Novo usuário:", string(j))

	postModel.Insert(ctx,
		&Post{Content: "primeiro post", UserID: john.ID},
		&Post{Content: "segundo post", UserID: john.ID},
	)

	// ========== READ ==========
	user, _ := userModel.FindOne(ctx, func(u *User, q *core.Query) { q.Eq(&u.Email, "john@example.com") })
	u, _ := json.Marshal(user)
	fmt.Println("User encontrado por email:", string(u))

	posts, _ := postModel.FindMany(ctx, func(p *Post, q *core.Query) {
		q.Eq(&p.UserID, john.ID)
		q.OrderBy(&p.CreatedAt, core.Desc)
	})
	p, _ := json.Marshal(posts)
	fmt.Println("Posts do John:", string(p))

	// ========== UPDATE ==========
	postModel.Update(ctx, func(p *Post, u *core.Update, q *core.Query) {
		u.Set(&p.Content, "conteúdo atualizado")
		q.Eq(&p.ID, 1)
	})

	// ========== DELETE ==========
	postModel.Delete(ctx, func(p *Post, q *core.Query) {
		q.Like(&p.Content, "%segundo%")
	})

	// ========== COUNT ==========
	count, _ := postModel.Count(ctx, func(p *Post, q *core.Query) { q.Eq(&p.UserID, john.ID) })
	fmt.Println("Posts restantes do John:", count)

	// ========== RAW QUERIES ==========
	userRows, _ := client.Query(ctx, `SELECT * FROM "user"`)
	typedRows, _ := core.AsType[User](userRows)
	tr, _ := json.Marshal(typedRows)
	fmt.Println("Raw query AsType ->", string(tr))

	postRows, _ := client.Query(ctx, `SELECT * FROM "post"`)
	jr, _ := core.AsJson(postRows)
	fmt.Println("Raw query AsJson ->", string(jr[0]))
}
