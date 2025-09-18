package mongo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/leandroluk/golem/core"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mopt "go.mongodb.org/mongo-driver/mongo/options"
)

// region mongoTransaction

type mongoTransaction struct {
	session mongo.Session
}

func (t *mongoTransaction) Commit(ctx context.Context) error {
	defer t.session.EndSession(ctx)
	return t.session.CommitTransaction(ctx)
}

func (t *mongoTransaction) Rollback(ctx context.Context) error {
	defer t.session.EndSession(ctx)
	return t.session.AbortTransaction(ctx)
}

//endregion

//region MongoDriver

type MongoDriver struct {
	client          *mongo.Client
	defaultDatabase string
}

var _ core.Driver = (*MongoDriver)(nil)

func NewMongoDriver(ctx context.Context, uri string, defaultDB string) (*MongoDriver, error) {
	opts := mopt.Client().ApplyURI(uri)
	opts.SetConnectTimeout(10 * time.Second).SetServerSelectionTimeout(10 * time.Second)
	cli, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := cli.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return &MongoDriver{client: cli, defaultDatabase: defaultDB}, nil
}

func (d *MongoDriver) dbFor(schema *core.SchemaCore) *mongo.Database {
	dbName := d.defaultDatabase
	if schema.Database != "" {
		dbName = schema.Database
	}
	if dbName == "" {
		panic("mongo driver: database name is empty (defina no Schema.Database ou no NewMongoDriver)")
	}
	return d.client.Database(dbName)
}

func (d *MongoDriver) coll(schema *core.SchemaCore) *mongo.Collection {
	if schema.Collection == "" {
		panic("mongo driver: Collection (collection) vazio no Schema")
	}
	return d.dbFor(schema).Collection(schema.Collection)
}

// --- helper para extrair SessionContext do ctx ---
func (d *MongoDriver) withSession(ctx context.Context) context.Context {
	if tx := core.TransactionFrom(ctx); tx != nil {
		if mt, ok := tx.(*mongoTransaction); ok {
			return mongo.NewSessionContext(ctx, mt.session)
		}
	}
	return ctx
}

func (d *MongoDriver) buildFilter(c *core.Condition) bson.M {
	if c == nil {
		return bson.M{}
	}
	if len(c.Children) > 0 {
		childFilters := make([]bson.M, 0, len(c.Children))
		for _, ch := range c.Children {
			childFilters = append(childFilters, d.buildFilter(ch))
		}
		switch *c.Operator {
		case core.OpAnd:
			return bson.M{"$and": childFilters}
		case core.OpOr:
			return bson.M{"$or": childFilters}
		case core.OpNot:
			return bson.M{"$nor": childFilters}
		default:
			return bson.M{}
		}
	}

	field := c.FieldName
	switch *c.Operator {
	case core.OpNil:
		return bson.M{field: bson.M{"$eq": nil}}
	case core.OpEq:
		return bson.M{field: c.Value}
	case core.OpGt:
		return bson.M{field: bson.M{"$gt": c.Value}}
	case core.OpGte:
		return bson.M{field: bson.M{"$gte": c.Value}}
	case core.OpLt:
		return bson.M{field: bson.M{"$lt": c.Value}}
	case core.OpLte:
		return bson.M{field: bson.M{"$lte": c.Value}}
	case core.OpLike:
		pat := toMongoLikePattern(fmt.Sprintf("%v", c.Value))
		return bson.M{field: primitive.Regex{Pattern: pat, Options: "i"}}
	case core.OpIn:
		var arr []any
		switch v := c.Value.(type) {
		case []any:
			arr = v
		default:
			arr = []any{c.Value}
		}
		return bson.M{field: bson.M{"$in": arr}}
	default:
		return bson.M{}
	}
}

func (d *MongoDriver) Connect(ctx context.Context) error {
	return d.client.Ping(ctx, nil)
}

func (d *MongoDriver) Ping(ctx context.Context) error {
	return d.client.Ping(ctx, nil)
}

func (d *MongoDriver) Close(ctx context.Context) error {
	return d.client.Disconnect(ctx)
}

func (d *MongoDriver) Transaction(ctx context.Context) (core.Transaction, error) {
	session, err := d.client.StartSession()
	if err != nil {
		return nil, err
	}
	if err := session.StartTransaction(); err != nil {
		return nil, err
	}
	return &mongoTransaction{session: session}, nil
}

func (d *MongoDriver) Insert(ctx context.Context, schema *core.SchemaCore, documents ...any) error {
	if len(documents) == 0 {
		return nil
	}
	ctx = d.withSession(ctx)
	docs := make([]any, 0, len(documents))
	docs = append(docs, documents...)
	_, err := d.coll(schema).InsertMany(ctx, docs)
	return err
}

func (d *MongoDriver) find(ctx context.Context, schema *core.SchemaCore, options *core.Query, single bool) ([]map[string]any, error) {
	ctx = d.withSession(ctx)
	filter := d.buildFilter(safeCondition(options))
	findOpts := mopt.Find()

	if len(options.Sort) > 0 {
		sortDoc := bson.D{}
		for _, s := range options.Sort {
			dir := 1
			if s.Order < 0 {
				dir = -1
			}
			sortDoc = append(sortDoc, bson.E{Key: s.FieldName, Value: dir})
		}
		findOpts.SetSort(sortDoc)
	}

	if single {
		findOpts.SetLimit(1)
	} else {
		if options.Limit > 0 {
			l := int64(options.Limit)
			findOpts.SetLimit(l)
		}
		if options.Offset > 0 {
			o := int64(options.Offset)
			findOpts.SetSkip(o)
		}
	}

	cur, err := d.coll(schema).Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []map[string]any
	for cur.Next(ctx) {
		var m bson.M
		if err := cur.Decode(&m); err != nil {
			return nil, err
		}
		row := map[string]any(m)
		out = append(out, row)
		if single {
			break
		}
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *MongoDriver) FindOne(ctx context.Context, schema *core.SchemaCore, options *core.Query) (any, error) {
	rows, err := d.find(ctx, schema, options, true)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (d *MongoDriver) FindMany(ctx context.Context, schema *core.SchemaCore, options *core.Query) (any, error) {
	return d.find(ctx, schema, options, false)
}

func (d *MongoDriver) Update(ctx context.Context, schema *core.SchemaCore, condition *core.Condition, changes core.Changes) error {
	ctx = d.withSession(ctx)
	filter := d.buildFilter(condition)
	update := bson.M{"$set": changes}
	_, err := d.coll(schema).UpdateMany(ctx, filter, update)
	return err
}

func (d *MongoDriver) Delete(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) error {
	ctx = d.withSession(ctx)
	filter := d.buildFilter(condition)
	_, err := d.coll(schema).DeleteMany(ctx, filter)
	return err
}

func (d *MongoDriver) Count(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) (int64, error) {
	ctx = d.withSession(ctx)
	filter := d.buildFilter(condition)
	return d.coll(schema).CountDocuments(ctx, filter)
}

//endregion

//region Helpers

func toMongoLikePattern(input string) string {
	const percent = "__PERCENT__"
	const underscore = "__UNDERSCORE__"
	s := strings.ReplaceAll(input, "%", percent)
	s = strings.ReplaceAll(s, "_", underscore)
	s = regexp.QuoteMeta(s)
	s = strings.ReplaceAll(s, percent, ".*")
	s = strings.ReplaceAll(s, underscore, ".")
	return s
}

func safeCondition(q *core.Query) *core.Condition {
	if q == nil || q.Condition == nil {
		return &core.Condition{Operator: &core.OpAnd, Children: []*core.Condition{}}
	}
	return q.Condition
}

//endregion
