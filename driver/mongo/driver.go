package driver

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

//region mongoTransaction

type mongoTransaction struct {
	session mongo.Session
}

func (transaction *mongoTransaction) Commit(ctx context.Context) error {
	defer transaction.session.EndSession(ctx)
	return transaction.session.CommitTransaction(ctx)
}

func (transaction *mongoTransaction) Rollback(ctx context.Context) error {
	defer transaction.session.EndSession(ctx)
	return transaction.session.AbortTransaction(ctx)
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
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return &MongoDriver{client: client, defaultDatabase: defaultDB}, nil
}

func (driver *MongoDriver) dbFor(schema *core.SchemaCore) *mongo.Database {
	dbName := driver.defaultDatabase
	if schema.Database != "" {
		dbName = schema.Database
	}
	if dbName == "" {
		panic("mongo driver: database name is empty (defina no Schema.Database ou no NewMongoDriver)")
	}
	return driver.client.Database(dbName)
}

func (driver *MongoDriver) coll(schema *core.SchemaCore) *mongo.Collection {
	if schema.Collection == "" {
		panic("mongo driver: Collection (collection) vazio no Schema")
	}
	return driver.dbFor(schema).Collection(schema.Collection)
}

// --- helper para extrair SessionContext do ctx ---
func (driver *MongoDriver) withSession(ctx context.Context) context.Context {
	if tx := core.TransactionFrom(ctx); tx != nil {
		if mt, ok := tx.(*mongoTransaction); ok {
			return mongo.NewSessionContext(ctx, mt.session)
		}
	}
	return ctx
}

func (driver *MongoDriver) buildFilter(condition *core.Condition) bson.M {
	if condition == nil {
		return bson.M{}
	}
	if len(condition.Children) > 0 {
		childFilterList := make([]bson.M, 0, len(condition.Children))
		for _, child := range condition.Children {
			childFilterList = append(childFilterList, driver.buildFilter(child))
		}
		switch *condition.Operator {
		case core.OpAnd:
			return bson.M{"$and": childFilterList}
		case core.OpOr:
			return bson.M{"$or": childFilterList}
		case core.OpNot:
			return bson.M{"$nor": childFilterList}
		default:
			return bson.M{}
		}
	}

	fieldName := condition.FieldName
	switch *condition.Operator {
	case core.OpNil:
		return bson.M{fieldName: bson.M{"$eq": nil}}
	case core.OpEq:
		return bson.M{fieldName: condition.Value}
	case core.OpGt:
		return bson.M{fieldName: bson.M{"$gt": condition.Value}}
	case core.OpGte:
		return bson.M{fieldName: bson.M{"$gte": condition.Value}}
	case core.OpLt:
		return bson.M{fieldName: bson.M{"$lt": condition.Value}}
	case core.OpLte:
		return bson.M{fieldName: bson.M{"$lte": condition.Value}}
	case core.OpLike:
		pattern := toMongoLikePattern(fmt.Sprintf("%v", condition.Value))
		return bson.M{fieldName: primitive.Regex{Pattern: pattern, Options: "i"}}
	case core.OpIn:
		var array []any
		switch v := condition.Value.(type) {
		case []any:
			array = v
		default:
			array = []any{condition.Value}
		}
		return bson.M{fieldName: bson.M{"$in": array}}
	default:
		return bson.M{}
	}
}

func (driver *MongoDriver) Connect(ctx context.Context) error {
	return driver.client.Ping(ctx, nil)
}

func (driver *MongoDriver) Ping(ctx context.Context) error {
	return driver.client.Ping(ctx, nil)
}

func (driver *MongoDriver) Close(ctx context.Context) error {
	return driver.client.Disconnect(ctx)
}

func (driver *MongoDriver) Transaction(ctx context.Context) (core.Transaction, error) {
	session, err := driver.client.StartSession()
	if err != nil {
		return nil, err
	}
	if err := session.StartTransaction(); err != nil {
		return nil, err
	}
	return &mongoTransaction{session: session}, nil
}

func (driver *MongoDriver) Insert(ctx context.Context, schema *core.SchemaCore, documents ...any) error {
	if len(documents) == 0 {
		return nil
	}
	ctx = driver.withSession(ctx)
	documentList := make([]any, 0, len(documents))
	documentList = append(documentList, documents...)
	_, err := driver.coll(schema).InsertMany(ctx, documentList)
	return err
}

func (driver *MongoDriver) find(ctx context.Context, schema *core.SchemaCore, query *core.Where, single bool) ([]map[string]any, error) {
	ctx = driver.withSession(ctx)
	filter := driver.buildFilter(safeCondition(query))
	findOpts := mopt.Find()

	if len(query.Sort) > 0 {
		sortDoc := bson.D{}
		for _, sortItem := range query.Sort {
			direction := 1
			if sortItem.Order < 0 {
				direction = -1
			}
			sortDoc = append(sortDoc, bson.E{Key: sortItem.FieldName, Value: direction})
		}
		findOpts.SetSort(sortDoc)
	}

	if single {
		findOpts.SetLimit(1)
	} else {
		if query.Limit > 0 {
			limit := int64(query.Limit)
			findOpts.SetLimit(limit)
		}
		if query.Offset > 0 {
			offset := int64(query.Offset)
			findOpts.SetSkip(offset)
		}
	}

	cursor, err := driver.coll(schema).Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var resultList []map[string]any
	for cursor.Next(ctx) {
		var bsonMap bson.M
		if err := cursor.Decode(&bsonMap); err != nil {
			return nil, err
		}
		row := map[string]any(bsonMap)
		resultList = append(resultList, row)
		if single {
			break
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return resultList, nil
}

func (driver *MongoDriver) FindOne(ctx context.Context, schema *core.SchemaCore, query *core.Where) (any, error) {
	rowList, err := driver.find(ctx, schema, query, true)
	if err != nil {
		return nil, err
	}
	if len(rowList) == 0 {
		return nil, nil
	}
	return rowList[0], nil
}

func (driver *MongoDriver) FindMany(ctx context.Context, schema *core.SchemaCore, query *core.Where) (any, error) {
	return driver.find(ctx, schema, query, false)
}

func (driver *MongoDriver) Update(ctx context.Context, schema *core.SchemaCore, condition *core.Condition, changes core.Changes) error {
	ctx = driver.withSession(ctx)
	filter := driver.buildFilter(condition)
	update := bson.M{"$set": changes}
	_, err := driver.coll(schema).UpdateMany(ctx, filter, update)
	return err
}

func (driver *MongoDriver) Delete(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) error {
	ctx = driver.withSession(ctx)
	filter := driver.buildFilter(condition)
	_, err := driver.coll(schema).DeleteMany(ctx, filter)
	return err
}

func (driver *MongoDriver) Count(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) (int64, error) {
	ctx = driver.withSession(ctx)
	filter := driver.buildFilter(condition)
	return driver.coll(schema).CountDocuments(ctx, filter)
}

//endregion

//region Helpers

func toMongoLikePattern(input string) string {
	const percent = "__PERCENT__"
	const underscore = "__UNDERSCORE__"
	safe := strings.ReplaceAll(input, "%", percent)
	safe = strings.ReplaceAll(safe, "_", underscore)
	safe = regexp.QuoteMeta(safe)
	safe = strings.ReplaceAll(safe, percent, ".*")
	safe = strings.ReplaceAll(safe, underscore, ".")
	return safe
}

func safeCondition(query *core.Where) *core.Condition {
	if query == nil || query.Condition == nil {
		return &core.Condition{Operator: &core.OpAnd, Children: []*core.Condition{}}
	}
	return query.Condition
}

//endregion
