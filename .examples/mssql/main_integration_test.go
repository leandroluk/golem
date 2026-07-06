//go:build integration

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/mssql"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/join"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

// TestMain runs the schema migration once before any test in this package
func TestMain(m *testing.M) {
	dsn := resolveDSN()
	ds, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) {
		o.DSN = dsn
	}), golem.DataSourceName("TestMain"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: NewDataSource:", err)
		os.Exit(1)
	}
	if err := ds.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: Connect:", err)
		os.Exit(1)
	}
	ctx := context.Background()
	for _, stmt := range migration {
		if _, err := ds.Exec(ctx, stmt); err != nil {
			fmt.Fprintln(os.Stderr, "TestMain: migration:", err)
			os.Exit(1)
		}
	}
	ds.Close()
	os.Exit(m.Run())
}

func TestBlogExample_FullFlow(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) {
		o.DSN = dsn
	}), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()
	user, err := repository.Get(dataSource, UserEntity).Insert(ctx, &User{
		Name:  "John Doe",
		Email: "john.doe@email.com",
	})
	if err != nil {
		t.Fatalf("Insert(User) returned error: %v", err)
	}
	if user.ID == 0 {
		t.Fatalf("expected non-zero user.ID after Insert, got %d", user.ID)
	}

	posts, err := repository.Get(dataSource, PostEntity).InsertMany(ctx,
		&Post{OwnerUserID: user.ID, Title: "Hello, Golem!", Content: "My first post using golem."},
		&Post{OwnerUserID: user.ID, Title: "A Second Post", Content: "Another post, still using golem."},
	)
	if err != nil {
		t.Fatalf("InsertMany(Post) returned error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	for i, p := range posts {
		if p.ID == 0 {
			t.Fatalf("posts[%d].ID is zero, want non-zero", i)
		}
		if p.OwnerUserID != user.ID {
			t.Fatalf("posts[%d].OwnerUserID = %d, want %d", i, p.OwnerUserID, user.ID)
		}
	}

	categories, err := repository.Get(dataSource, CategoryEntity).InsertMany(ctx,
		&Category{Name: "Announcements"},
		&Category{Name: "Tutorials"},
	)
	if err != nil {
		t.Fatalf("InsertMany(Category) returned error: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(categories))
	}
	for i, c := range categories {
		if c.ID == 0 {
			t.Fatalf("categories[%d].ID is zero, want non-zero", i)
		}
	}

	if _, err := repository.Get(dataSource, PostToCategoryEntity).InsertMany(ctx,
		&PostToCategory{PostID: posts[0].ID, CategoryID: categories[0].ID},
		&PostToCategory{PostID: posts[1].ID, CategoryID: categories[1].ID},
	); err != nil {
		t.Fatalf("InsertMany(PostToCategory) returned error: %v", err)
	}

	foundUser, err := repository.Get(dataSource, UserEntity).FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.ID, user.ID))
	})
	if err != nil {
		t.Fatalf("FindOne(user.ID) returned error: %v", err)
	}
	if foundUser.Name != user.Name {
		t.Errorf("foundUser.Name = %q, want %q", foundUser.Name, user.Name)
	}
	if foundUser.Email != user.Email {
		t.Errorf("foundUser.Email = %q, want %q", foundUser.Email, user.Email)
	}

	_, err = repository.Get(dataSource, UserEntity).FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.ID, user.ID+1_000_000))
	})
	if !errors.Is(err, golem.ErrNotFound) {
		t.Errorf("FindOne(nonexistent ID): expected errors.Is(err, golem.ErrNotFound), got %v", err)
	}

	usersWithPosts, err := repository.Get(dataSource, UserEntity).FindMany(ctx, func(u *User, q0 *query.Query[User]) {
		join.Inner(q0, PostEntity, func(p *Post, q1 *query.Join[Post]) {
			q1.On(&p.OwnerUserID, &u.ID)
		})
		q0.Where(op.Eq(&u.ID, user.ID))
	})
	if err != nil {
		t.Fatalf("FindMany with join.Inner returned error: %v", err)
	}
	if len(usersWithPosts) != 1 {
		t.Errorf("expected 1 user with posts, got %d", len(usersWithPosts))
	} else if usersWithPosts[0].ID != user.ID {
		t.Errorf("expected user.ID %d, got %d", user.ID, usersWithPosts[0].ID)
	}
}

func TestBlogExample_PessimisticLocking_ForUpdateBlocksConcurrentLocker(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn }), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()
	user, err := repository.Get(dataSource, UserEntity).Insert(ctx, &User{
		Name:  "Lock User",
		Email: "lock@email.com",
	})
	if err != nil {
		t.Fatalf("Insert(User) returned error: %v", err)
	}

	lockedCh := make(chan struct{})
	releaseCh := make(chan struct{})
	tx2DoneCh := make(chan error, 1)

	go func() {
		_ = dataSource.Transaction(ctx, func(tx golem.Tx) error {
			userRepo := repository.Get(tx, UserEntity)
			_, err := userRepo.FindOne(ctx, func(u *User, q *query.Query[User]) {
				q.Where(op.Eq(&u.ID, user.ID))
				q.ForUpdate()
			})
			if err != nil {
				return err
			}
			close(lockedCh)
			<-releaseCh
			return nil
		})
	}()

	<-lockedCh

	go func() {
		err := dataSource.Transaction(ctx, func(tx golem.Tx) error {
			userRepo := repository.Get(tx, UserEntity)
			_, err := userRepo.FindOne(ctx, func(u *User, q *query.Query[User]) {
				q.Where(op.Eq(&u.ID, user.ID))
				q.ForUpdate()
			})
			return err
		})
		tx2DoneCh <- err
	}()

	time.Sleep(300 * time.Millisecond)

	select {
	case err := <-tx2DoneCh:
		t.Fatalf("tx2's FindOne+ForUpdate returned (err=%v) before tx1 released the lock", err)
	default:
	}

	close(releaseCh)

	select {
	case err := <-tx2DoneCh:
		if err != nil {
			t.Fatalf("tx2's FindOne+ForUpdate returned an error after tx1 released the lock: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tx2's FindOne+ForUpdate never returned after tx1 committed")
	}
}

func TestBlogExample_ForUpdate_OutsideTransaction_ReturnsError(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn }), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()
	userRepo := repository.Get(dataSource, UserEntity)

	_, err = userRepo.FindMany(ctx, func(u *User, q *query.Query[User]) {
		q.ForUpdate()
	})
	if err == nil {
		t.Fatal("expected FindMany+ForUpdate outside a transaction to return an error, got nil")
	}
}

type UserPostStats struct {
	OwnerUserID int64
	PostCount   int64
}

func TestBlogExample_Aggregate_PostCountPerUser(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn }), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()
	userRepo := repository.Get(dataSource, UserEntity)
	postRepo := repository.Get(dataSource, PostEntity)

	user, err := userRepo.Insert(ctx, &User{Name: "Aggregate User", Email: "aggregate@email.com"})
	if err != nil {
		t.Fatalf("Insert(User) returned error: %v", err)
	}
	if _, err := postRepo.InsertMany(ctx,
		&Post{OwnerUserID: user.ID, Title: "P1", Content: "..."},
		&Post{OwnerUserID: user.ID, Title: "P2", Content: "..."},
		&Post{OwnerUserID: user.ID, Title: "P3", Content: "..."},
	); err != nil {
		t.Fatalf("InsertMany(Post) returned error: %v", err)
	}

	stats, err := repository.Aggregate(ctx, postRepo, func(p *Post, res *UserPostStats, a *query.Aggregate[Post, UserPostStats]) {
		a.GroupBy(&p.OwnerUserID, &res.OwnerUserID)
		a.CountAll(&res.PostCount)
		a.Where(op.Eq(&p.OwnerUserID, user.ID))
		a.Having(op.Gt(&res.PostCount, int64(0)))
	})
	if err != nil {
		t.Fatalf("Aggregate returned error: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 grouped result, got %d", len(stats))
	}
	if stats[0].OwnerUserID != user.ID {
		t.Errorf("OwnerUserID = %d, want %d", stats[0].OwnerUserID, user.ID)
	}
	if stats[0].PostCount != 3 {
		t.Errorf("PostCount = %d, want 3", stats[0].PostCount)
	}
}

func TestBlogExample_Preload_LoadsPostsPerUser(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn }), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()
	userRepo := repository.Get(dataSource, UserEntity)
	postRepo := repository.Get(dataSource, PostEntity)

	userA, err := userRepo.Insert(ctx, &User{Name: "Preload User A", Email: "preload.a@email.com"})
	if err != nil {
		t.Fatalf("Insert(User A) returned error: %v", err)
	}
	userB, err := userRepo.Insert(ctx, &User{Name: "Preload User B", Email: "preload.b@email.com"})
	if err != nil {
		t.Fatalf("Insert(User B) returned error: %v", err)
	}

	if _, err := postRepo.InsertMany(ctx,
		&Post{OwnerUserID: userA.ID, Title: "A1", Content: "..."},
		&Post{OwnerUserID: userA.ID, Title: "A2", Content: "..."},
		&Post{OwnerUserID: userB.ID, Title: "B1", Content: "..."},
	); err != nil {
		t.Fatalf("InsertMany(Post) returned error: %v", err)
	}

	users, err := userRepo.FindMany(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Or(op.Eq(&u.ID, userA.ID), op.Eq(&u.ID, userB.ID)))
	})
	if err != nil {
		t.Fatalf("FindMany(User) returned error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	postsByUser, err := repository.Preload(ctx, userRepo, users, PostEntity)
	if err != nil {
		t.Fatalf("Preload returned error: %v", err)
	}
	if len(postsByUser[userA.ID]) != 2 {
		t.Errorf("postsByUser[userA.ID] = %d posts, want 2", len(postsByUser[userA.ID]))
	}
	if len(postsByUser[userB.ID]) != 1 {
		t.Errorf("postsByUser[userB.ID] = %d posts, want 1", len(postsByUser[userB.ID]))
	}
}

func TestBlogExample_CascadeDeleteUser_DeletesTheirPosts(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn }), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()
	userRepo := repository.Get(dataSource, UserEntity)
	postRepo := repository.Get(dataSource, PostEntity)

	user, err := userRepo.Insert(ctx, &User{
		Name:  "Cascade Delete User",
		Email: "cascade.delete@email.com",
	})
	if err != nil {
		t.Fatalf("Insert(User) returned error: %v", err)
	}

	post, err := postRepo.Insert(ctx, &Post{
		OwnerUserID: user.ID,
		Title:       "Post that should be cascade-deleted",
		Content:     "deleting the owning user should delete this row too",
	})
	if err != nil {
		t.Fatalf("Insert(Post) returned error: %v", err)
	}

	if err := userRepo.Delete(ctx, &user); err != nil {
		t.Fatalf("Delete(User) returned error: %v", err)
	}

	_, err = userRepo.FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.ID, user.ID))
	})
	if !errors.Is(err, golem.ErrNotFound) {
		t.Errorf("expected deleted user to be ErrNotFound, got %v", err)
	}

	_, err = postRepo.FindOne(ctx, func(p *Post, q *query.Query[Post]) {
		q.Where(op.Eq(&p.ID, post.ID))
	})
	if !errors.Is(err, golem.ErrNotFound) {
		t.Errorf("expected cascade-deleted post to be ErrNotFound, got %v", err)
	}
}

func TestBlogExample_TypedErrors(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn }), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()

	user, err := repository.Get(dataSource, UserEntity).Insert(ctx, &User{
		Name:  "Typed Errors User",
		Email: "typed.errors@email.com",
	})
	if err != nil {
		t.Fatalf("Insert(User) returned error: %v", err)
	}

	_, err = repository.Get(dataSource, PostEntity).Insert(ctx, &Post{
		OwnerUserID: user.ID + 1_000_000,
		Title:       "Orphan Post",
		Content:     "references a user that doesn't exist",
	})
	if !errors.Is(err, golem.ErrForeignKeyViolation) {
		t.Errorf("Insert(Post) with bad OwnerUserID: expected errors.Is(err, golem.ErrForeignKeyViolation), got %v", err)
	}

	post, err := repository.Get(dataSource, PostEntity).Insert(ctx, &Post{
		OwnerUserID: user.ID,
		Title:       "Typed Errors Post",
		Content:     "used to trigger a duplicate key below",
	})
	if err != nil {
		t.Fatalf("Insert(Post) returned error: %v", err)
	}

	category, err := repository.Get(dataSource, CategoryEntity).Insert(ctx, &Category{Name: "Typed Errors Category"})
	if err != nil {
		t.Fatalf("Insert(Category) returned error: %v", err)
	}

	ptcRepo := repository.Get(dataSource, PostToCategoryEntity)
	if _, err := ptcRepo.Insert(ctx, &PostToCategory{PostID: post.ID, CategoryID: category.ID}); err != nil {
		t.Fatalf("Insert(PostToCategory) returned error: %v", err)
	}

	_, err = ptcRepo.Insert(ctx, &PostToCategory{PostID: post.ID, CategoryID: category.ID})
	if !errors.Is(err, golem.ErrDuplicateKey) {
		t.Errorf("Insert(PostToCategory) duplicate PK: expected errors.Is(err, golem.ErrDuplicateKey), got %v", err)
	}
}

type TempPost struct {
	ID        int64
	Title     string
	DeletedAt *time.Time
}

var TempPostEntity = entity.New[TempPost](func(t *TempPost, b *entity.Table) {
	b.TableName("temp_posts")
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Title, golem.VARCHAR(50))
	b.Col(&t.DeletedAt, golem.DATETIME()).Name("deleted_at")
	b.PrimaryKey(&t.ID)
	b.DeleteDate(&t.DeletedAt)
})

func TestBlogExample_DeleteCountAndExists(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn }), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()

	// MSSQL: creating a normal table, tests run fast enough, and container is discarded
	_, err = dataSource.Dialect().Exec(ctx, dataSource, `
	IF OBJECT_ID('temp_posts', 'U') IS NOT NULL DROP TABLE temp_posts;
	CREATE TABLE temp_posts (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		title VARCHAR(50) NOT NULL,
		deleted_at DATETIMEOFFSET
	)`, nil)
	if err != nil {
		t.Fatalf("failed to create temporary table: %v", err)
	}
	defer dataSource.Dialect().Exec(ctx, dataSource, `DROP TABLE temp_posts`, nil)

	repo := repository.Get(dataSource, TempPostEntity)

	inserted, err := repo.InsertMany(ctx,
		&TempPost{Title: "Post 1"},
		&TempPost{Title: "Post 2"},
	)
	if err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	if len(inserted) != 2 {
		t.Fatalf("expected 2 inserted posts")
	}

	cnt, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if cnt != 2 {
		t.Errorf("Count = %d, want 2", cnt)
	}

	hasRows, err := repo.Exists(ctx, func(p *TempPost, c *query.Count[TempPost]) {
		c.Where(op.Eq(&p.Title, "Post 1"))
	})
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !hasRows {
		t.Errorf("expected Exists to be true")
	}

	err = repo.Delete(ctx, &inserted[0])
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	cnt, err = repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count after delete: %v", err)
	}
	if cnt != 1 {
		t.Errorf("Count after delete = %d, want 1", cnt)
	}

	cnt, err = repo.Count(ctx, func(p *TempPost, c *query.Count[TempPost]) {
		c.WithDeleted()
	})
	if err != nil {
		t.Fatalf("Count with deleted: %v", err)
	}
	if cnt != 2 {
		t.Errorf("Count with deleted = %d, want 2", cnt)
	}

	err = repo.Restore(ctx, &inserted[0])
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	cnt, err = repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count after restore: %v", err)
	}
	if cnt != 2 {
		t.Errorf("Count after restore = %d, want 2", cnt)
	}
}

func TestBlogExample_Transactions(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn }), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()

	err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
		userRepo := repository.Get(tx, UserEntity)
		_, err := userRepo.Insert(ctx, &User{
			Name:  "Tx User Success",
			Email: "tx.success@email.com",
		})
		return err
	})
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	userRepo := repository.Get(dataSource, UserEntity)
	dbUser, err := userRepo.FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.Email, "tx.success@email.com"))
	})
	if err != nil {
		t.Fatalf("Failed to find committed user: %v", err)
	}
	if dbUser.Name != "Tx User Success" {
		t.Errorf("expected user name 'Tx User Success', got %q", dbUser.Name)
	}

	txErr := errors.New("abort transaction")
	err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
		userRepoTx := repository.Get(tx, UserEntity)
		_, err := userRepoTx.Insert(ctx, &User{
			Name:  "Tx User Failed",
			Email: "tx.failed@email.com",
		})
		if err != nil {
			return err
		}
		return txErr
	})
	if !errors.Is(err, txErr) {
		t.Fatalf("expected transaction to return txErr, got %v", err)
	}

	_, err = userRepo.FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.Email, "tx.failed@email.com"))
	})
	if !errors.Is(err, golem.ErrNotFound) {
		t.Errorf("expected ErrNotFound for rolled back user, got %v", err)
	}

	defer func() {
		r := recover()
		if r == nil || r.(string) != "panic in tx" {
			t.Fatalf("expected panic 'panic in tx', got %v", r)
		}

		_, err = userRepo.FindOne(ctx, func(u *User, q *query.Query[User]) {
			q.Where(op.Eq(&u.Email, "tx.panic@email.com"))
		})
		if !errors.Is(err, golem.ErrNotFound) {
			t.Errorf("expected ErrNotFound for panic-rolled-back user, got %v", err)
		}
	}()

	_ = dataSource.Transaction(ctx, func(tx golem.Tx) error {
		userRepoTx := repository.Get(tx, UserEntity)
		_, _ = userRepoTx.Insert(ctx, &User{
			Name:  "Tx User Panic",
			Email: "tx.panic@email.com",
		})
		panic("panic in tx")
	})
}

func TestBlogExample_RawExec(t *testing.T) {
	dsn := resolveDSN()
	dataSource, err := golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn }), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()
	userRepo := repository.Get(dataSource, UserEntity)
	u, err := userRepo.Insert(ctx, &User{
		Name:  "Raw Exec User",
		Email: "raw.exec@email.com",
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	result, err := dataSource.Exec(ctx, "UPDATE users SET name = @p1 WHERE id = @p2", "Raw Exec User Updated", u.ID)
	if err != nil {
		t.Fatalf("DataSource.Exec: %v", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}

	checkRes, err := dataSource.Exec(ctx, "SELECT name FROM users WHERE id = @p1", u.ID)
	if err != nil {
		t.Fatalf("DataSource.Exec for SELECT: %v", err)
	}
	var scannedNames []string
	for checkRes.Next() {
		row, err := checkRes.Scan()
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		scannedNames = append(scannedNames, row["name"].(string))
	}
	if len(scannedNames) != 1 || scannedNames[0] != "Raw Exec User Updated" {
		t.Errorf("unexpected scanned rows: %v", scannedNames)
	}

	// 2. Repository.Exec SELECT
	users, err := userRepo.Exec(ctx, "SELECT * FROM users WHERE email = @p1", "raw.exec@email.com")
	if err != nil {
		t.Fatalf("Repository.Exec: %v", err)
	}

	if len(users) != 1 || users[0].Name != "Raw Exec User Updated" {
		t.Errorf("unexpected mapped users: %+v", users)
	}
}
