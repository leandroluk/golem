package core

import "errors"

// ErrDuplicateKey indicates a write violated a unique constraint (single or composite).
var ErrDuplicateKey = errors.New("golem: duplicate key")

// ErrForeignKeyViolation indicates a write violated a foreign key constraint
// (referenced row missing, or deleting a row still referenced elsewhere).
var ErrForeignKeyViolation = errors.New("golem: foreign key violation")

// ErrDataSourceNotFound indicates GetDataSource was called with a name no
// NewDataSource call has registered (or one already Close()'d).
var ErrDataSourceNotFound = errors.New("golem: data source not found")
