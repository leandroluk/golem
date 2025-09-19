// Package driver provides database driver implementations for the golem ORM.
// This file implements the MongoDB driver, which adapts the core.Driver interface
// to work with the official MongoDB Go driver.
package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/leandroluk/golem/core"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mopt "go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDriver is the driver implementation for MongoDB.
//
// It wraps the official MongoDB Go client and implements the core.Driver
// interface, supporting inserts, queries, updates, deletes, counting,
// and transaction sessions.
type MongoDriver struct {
	client          *mongo.Client
	defaultDatabase string
}

// Ensure MongoDriver implements core.Driver at compile time.
var _ core.Driver = (*MongoDriver)(nil)

// NewMongoDriver creates a new MongoDriver given a MongoDB URI and a default database.
//
// It sets reasonable connection and server selection timeouts and ensures
// connectivity by pinging the server.
//
// Example:
//
//	driver, err := driver.NewMongoDriver(ctx, "mongodb://localhost:27017", "mydb")
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

// dbFor returns the *mongo.Database for the given schema,
// falling back to the driver's default database if none is specified.
func (driver *MongoDriver) dbFor(schema *core.SchemaCore) *mongo.Database {
	dbName := driver.defaultDatabase
	if schema.Database != "" {
		dbName = schema.Database
	}
	if dbName == "" {
		panic("mongo driver: database name is empty (set it in Schema.Database or in NewMongoDriver)")
	}
	return driver.client.Database(dbName)
}

// coll returns the *mongo.Collection for the given schema.
func (driver *MongoDriver) coll(schema *core.SchemaCore) *mongo.Collection {
	if schema.Collection == "" {
		panic("mongo driver: collection name is empty in Schema")
	}
	return driver.dbFor(schema).Collection(schema.Collection)
}

// withSession attaches a MongoDB session to the context if a transaction
// is currently active. Otherwise, it returns the original context.
func (driver *MongoDriver) withSession(ctx context.Context) context.Context {
	if tx := core.TransactionFrom(ctx); tx != nil {
		if mt, ok := tx.(*mongoTransaction); ok {
			return mongo.NewSessionContext(ctx, mt.session)
		}
	}
	return ctx
}

// buildFilter converts a core.Condition into a MongoDB filter (bson.M).
//
// It supports logical operators (AND, OR, NOT) as well as value operators
// (Eq, Gt, Gte, Lt, Lte, Like, In, Nil).
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

// Connect verifies connectivity with the MongoDB server.
func (driver *MongoDriver) Connect(ctx context.Context) error {
	return driver.client.Ping(ctx, nil)
}

// Ping checks if the MongoDB server is reachable.
func (driver *MongoDriver) Ping(ctx context.Context) error {
	return driver.client.Ping(ctx, nil)
}

// Close disconnects the MongoDB client and releases resources.
func (driver *MongoDriver) Close(ctx context.Context) error {
	return driver.client.Disconnect(ctx)
}

// Transaction starts a new MongoDB session and transaction.
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

// Insert inserts one or more documents into the collection defined by the schema.
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

// find executes a query and returns the matching documents as a list of maps.
//
// If single is true, it limits the result to one document.
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

// FindOne retrieves a single document matching the given query.
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

// FindMany retrieves all documents matching the given query.
func (driver *MongoDriver) FindMany(ctx context.Context, schema *core.SchemaCore, query *core.Where) (any, error) {
	return driver.find(ctx, schema, query, false)
}

// Update applies changes to all documents matching the given condition.
func (driver *MongoDriver) Update(ctx context.Context, schema *core.SchemaCore, condition *core.Condition, changes core.Changes) error {
	ctx = driver.withSession(ctx)
	filter := driver.buildFilter(condition)
	update := bson.M{"$set": changes}
	_, err := driver.coll(schema).UpdateMany(ctx, filter, update)
	return err
}

// Delete removes all documents matching the given condition.
func (driver *MongoDriver) Delete(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) error {
	ctx = driver.withSession(ctx)
	filter := driver.buildFilter(condition)
	_, err := driver.coll(schema).DeleteMany(ctx, filter)
	return err
}

// Count returns the number of documents matching the given condition.
func (driver *MongoDriver) Count(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) (int64, error) {
	ctx = driver.withSession(ctx)
	filter := driver.buildFilter(condition)
	return driver.coll(schema).CountDocuments(ctx, filter)
}
