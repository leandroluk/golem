// Package driver provides database driver implementations for the golem ORM.
// This file contains helper functions used by the MongoDB driver for
// query translation and safety checks.
package driver

import (
	"regexp"
	"strings"

	"github.com/leandroluk/golem/core"
)

// toMongoLikePattern converts a SQL-like pattern into a MongoDB regex pattern.
//
// It replaces % with .* (wildcard for multiple characters) and
// _ with . (wildcard for a single character).
//
// Example:
//
//	input := "%admin_"
//	regex := toMongoLikePattern(input)
//	// regex == ".*admin."
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

// safeCondition ensures that a Where clause always has a valid root condition.
//
// If the query or its Condition is nil, it returns an empty AND condition.
// This prevents drivers from having to handle nil pointers explicitly.
//
// Example:
//
//	cond := safeCondition(nil)
//	// cond is &Condition{Operator: &core.OpAnd, Children: []}
func safeCondition(query *core.Where) *core.Condition {
	if query == nil || query.Condition == nil {
		return &core.Condition{Operator: &core.OpAnd, Children: []*core.Condition{}}
	}
	return query.Condition
}
